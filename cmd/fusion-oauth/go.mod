module github.com/PivotLLM/MCPFusion/cmd/fusion-oauth

go 1.24.2

toolchain go1.24.6

require (
	github.com/PivotLLM/MCPFusion v0.0.0
	golang.org/x/crypto v0.32.0
)

replace github.com/PivotLLM/MCPFusion => ../../

require (
	github.com/google/uuid v1.6.0 // indirect
	go.etcd.io/bbolt v1.4.2 // indirect
	golang.org/x/sys v0.35.0 // indirect
)
