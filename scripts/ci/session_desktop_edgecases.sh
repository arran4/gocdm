#!/usr/bin/env bash
set -euo pipefail

go test ./session -run 'TestParseDesktopFile(WhitespaceAroundEquals|LocalizedNameFallback|QuotedExecPreserved)$'
