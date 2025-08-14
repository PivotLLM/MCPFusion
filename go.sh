#!/bin/sh
#===============================================================================
# Copyright (c) 2025 Tenebris Technologies Inc.                                =
# All rights reserved.                                                         =
#===============================================================================

rm MCPFusion
go build -o MCPFusion
#./MCPFusion -config configs/microsoft365.json -port 8888
./MCPFusion
