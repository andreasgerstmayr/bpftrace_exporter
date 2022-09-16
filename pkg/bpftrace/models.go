package bpftrace

import (
	"encoding/json"
)

const (
	VarTypeNumber = iota
	VarTypeHistogram
)

type Output struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}
type AttachedProbesData struct {
	Probes int `json:"probes"`
}
type VarData = map[string]json.RawMessage

type Map = map[string]json.RawMessage

type Number = float64

type HistBucket struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Count uint64  `json:"count"`
}
type Hist = []HistBucket
