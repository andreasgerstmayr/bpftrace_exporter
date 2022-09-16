# bpftrace-exporter
Exports variables from [bpftrace](https://github.com/iovisor/bpftrace) scripts as metrics.

## Requirements
* bpftrace v0.15.0+

## Usage
```
./bpftrace_exporter -script bpftrace_script.bt -vars var1:type1,var2:type2,...
```

Example:
```
./bpftrace_exporter -script /usr/share/bpftrace/tools/runqlat.bt -vars usecs:hist
```

`vars` is a comma-separated list of bpftrace variable names (without `@`) and their type.

### Supported bpftrace variable types
Type | Description | bpftrace example
-----|-------------|-----------------
(empty) | scalar value | `@var = 5;`
counter | counter value | `@var = count();`
map | key/value map | `@var[pid] = 1;`
countermap | map with counter values | `@var[pid] = count();`
hist | histogram | `@var = hist(retval);`
histmap | keyed histogram | `@var[comm] = hist(retval);`

## Internals
On every scrape the bpftrace exporter sends a `SIGUSR1` signal to the bpftrace process, which prints all bpftrace variables in JSON format to stdout.
The exporter parses the output and emits metrics in the [OpenMetrics](https://openmetrics.io) format.

## Related Projects
* Performance Co-Pilot [bpf PMDA](https://github.com/performancecopilot/pcp/blob/main/src/pmdas/bpf/README), [bcc PMDA](https://github.com/performancecopilot/pcp/blob/main/src/pmdas/bcc/README.md), [bpftrace PMDA](https://github.com/performancecopilot/pcp/blob/main/src/pmdas/bpftrace/README.md)
* [ebpf_exporter](https://github.com/cloudflare/ebpf_exporter)

## License
Apache License 2.0, see [LICENSE](LICENSE).
