#!/bin/sh
rm MCPFusion
go build -o MCPFusion
./MCPFusion -fusion-config fusion/configs/microsoft365.json -port 8888
