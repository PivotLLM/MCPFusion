#!/bin/sh
#*******************************************************************************
# Copyright (c) 2025-2026 Tenebris Technologies Inc.                           *
# Please see LICENSE file for details.                                         *
#*******************************************************************************

rm MCPFusion
go build -o MCPFusion
./MCPFusion -debug -no-auth
