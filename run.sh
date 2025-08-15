#!/bin/sh
rm MCPFusion
go build -o MCPFusion
#./MCPFusion -config configs/microsoft365.json -port 8888
./MCPFusion
