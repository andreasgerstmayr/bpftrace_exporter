build:
	go build

test:
	go test ./...

run-runqlat: build
	./bpftrace_exporter -script /usr/share/bpftrace/tools/runqlat.bt -vars usecs:hist

clean:
	rm -f bpftrace_exporter
