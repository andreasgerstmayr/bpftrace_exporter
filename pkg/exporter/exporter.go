package exporter

import (
	"encoding/json"
	"fmt"
	"log"
	"path"
	"strings"
	"sync"

	"github.com/andreasgerstmayr/bpftrace_exporter/pkg/bpftrace"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "bpftrace"
)

type Exporter struct {
	mutex         sync.RWMutex
	numProbesDesc *prometheus.Desc

	process    *bpftrace.Process
	scriptName string
	vars       map[string]*VarDef
}

type VarDef struct {
	VarType  int
	IsMap    bool
	Desc     *prometheus.Desc
	PromType prometheus.ValueType
}

func NewExporter(bpftracePath string, scriptPath string, varDefs string) (*Exporter, error) {
	process, err := bpftrace.NewProcess(bpftracePath, scriptPath)
	if err != nil {
		return nil, err
	}

	err = process.Start()
	if err != nil {
		return nil, err
	}

	scriptName := strings.Split(path.Base(scriptPath), ".")[0]
	numProbesDesc := prometheus.NewDesc(prometheus.BuildFQName(namespace, scriptName, "probes_total"), "number of attached probes", nil, nil)

	vars, err := parseVarDefs(scriptName, varDefs)
	if err != nil {
		return nil, err
	}

	return &Exporter{
		numProbesDesc: numProbesDesc,
		process:       process,
		scriptName:    scriptName,
		vars:          vars,
	}, nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.numProbesDesc
	for _, metric := range e.vars {
		ch <- metric.Desc
	}
}
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	e.scrape(ch)
}

func parseVarDefs(scriptName string, varDefs string) (map[string]*VarDef, error) {
	vars := map[string]*VarDef{}
	for _, varDef := range strings.Split(varDefs, ",") {
		if varDef == "" {
			continue
		}

		// example: `usecs:hist`
		s := strings.Split(varDef, ":")
		name := s[0]
		def := ""
		if len(s) > 1 {
			def = s[1]
		}

		switch def {
		case "":
			vars[name] = &VarDef{
				VarType:  bpftrace.VarTypeNumber,
				IsMap:    false,
				Desc:     prometheus.NewDesc(prometheus.BuildFQName(namespace, scriptName, name), fmt.Sprintf("bpftrace variable @%s", name), nil, nil),
				PromType: prometheus.GaugeValue,
			}
		case "counter":
			vars[name] = &VarDef{
				VarType:  bpftrace.VarTypeNumber,
				IsMap:    false,
				Desc:     prometheus.NewDesc(prometheus.BuildFQName(namespace, scriptName, name), fmt.Sprintf("bpftrace variable @%s", name), nil, nil),
				PromType: prometheus.CounterValue,
			}
		case "map":
			vars[name] = &VarDef{
				VarType:  bpftrace.VarTypeNumber,
				IsMap:    true,
				Desc:     prometheus.NewDesc(prometheus.BuildFQName(namespace, scriptName, name), fmt.Sprintf("bpftrace map @%s", name), []string{"key"}, nil),
				PromType: prometheus.GaugeValue,
			}
		case "countermap":
			vars[name] = &VarDef{
				VarType:  bpftrace.VarTypeNumber,
				IsMap:    true,
				Desc:     prometheus.NewDesc(prometheus.BuildFQName(namespace, scriptName, name), fmt.Sprintf("bpftrace map @%s", name), []string{"key"}, nil),
				PromType: prometheus.CounterValue,
			}
		case "hist":
			vars[name] = &VarDef{
				VarType: bpftrace.VarTypeHistogram,
				IsMap:   false,
				Desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, scriptName, name), fmt.Sprintf("bpftrace histogram @%s", name), nil, nil),
			}
		case "histmap":
			vars[name] = &VarDef{
				VarType: bpftrace.VarTypeHistogram,
				IsMap:   true,
				Desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, scriptName, name), fmt.Sprintf("bpftrace histogram @%s", name), []string{"key"}, nil),
			}
		default:
			return nil, fmt.Errorf("unknown variable definition: \"%s\"", def)
		}
	}
	return vars, nil
}

func exportNumber(ch chan<- prometheus.Metric, varDef *VarDef, val json.RawMessage, labelValues ...string) {
	var bpfNum bpftrace.Number
	err := json.Unmarshal(val, &bpfNum)
	if err != nil {
		log.Printf("cannot unmarshal %s into a number: %v", val, err)
		return
	}

	ch <- prometheus.MustNewConstMetric(varDef.Desc, varDef.PromType, bpfNum, labelValues...)
}

func exportHist(ch chan<- prometheus.Metric, varDef *VarDef, val json.RawMessage, labelValues ...string) {
	var bpfHist bpftrace.Hist
	err := json.Unmarshal(val, &bpfHist)
	if err != nil {
		log.Printf("cannot unmarshal %s into a histogram: %v", val, err)
		return
	}

	buckets := map[float64]uint64{}
	count := uint64(0)

	// bpftrace buckets are already sorted
	for _, bucket := range bpfHist {
		upperBound := bucket.Max
		count += bucket.Count
		buckets[upperBound] = count
	}
	ch <- prometheus.MustNewConstHistogram(varDef.Desc, count, -1, buckets, labelValues...)
}

func exportScalarVar(ch chan<- prometheus.Metric, varDef *VarDef, val json.RawMessage, labelValues ...string) {
	switch varDef.VarType {
	case bpftrace.VarTypeNumber:
		exportNumber(ch, varDef, val, labelValues...)
	case bpftrace.VarTypeHistogram:
		exportHist(ch, varDef, val, labelValues...)
	}
}

func exportVar(ch chan<- prometheus.Metric, varDef *VarDef, val json.RawMessage) {
	if !varDef.IsMap {
		exportScalarVar(ch, varDef, val)
	} else {
		var mapData bpftrace.Map
		err := json.Unmarshal(val, &mapData)
		if err != nil {
			log.Printf("cannot unmarshal %s into a map: %v", val, err)
			return
		}

		for k, v := range mapData {
			exportScalarVar(ch, varDef, v, k)
		}
	}
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(e.numProbesDesc, prometheus.GaugeValue, float64(e.process.NumProbes))

	err := e.process.SendSigusr1()
	if err != nil {
		log.Printf("error sending signal: %v", err)
		return
	}

	var out bpftrace.Output
	// finish loop once all variables are processed
	for remVars := len(e.vars); remVars > 0 && e.process.StdoutScanner.Scan(); {
		line := e.process.StdoutScanner.Text()
		err := json.Unmarshal([]byte(line), &out)
		if err != nil {
			log.Printf("cannot parse JSON of line %s: %v", line, err)
			return
		}

		switch out.Type {
		case "printf":
			log.Printf("bpftrace output: %s", out.Data)
		case "map", "hist":
			var varData bpftrace.VarData
			err := json.Unmarshal(out.Data, &varData)
			if err != nil {
				log.Printf("cannot parse JSON of line %s: %v", line, err)
				return
			}

			for name, val := range varData {
				varDef, ok := e.vars[name[1:]]
				if ok {
					exportVar(ch, varDef, val)
					remVars--
				}
			}
		default:
			log.Printf("unknown output type: %s", out.Type)
		}
	}
}

func (e *Exporter) Stop() error {
	return e.process.SendSigint()
}
