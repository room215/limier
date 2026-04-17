#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
OUTPUT_DIR=${LIMIER_OUTPUT_DIR:-"$ROOT_DIR/out/limier"}
REPORT_PATH="$OUTPUT_DIR/report.json"
SUMMARY_PATH="$OUTPUT_DIR/summary.md"
RENDER_FORMAT=${LIMIER_RENDER_FORMAT:-build-summary}
RENDER_OUTPUT=${LIMIER_RENDER_OUTPUT:-"$OUTPUT_DIR/build-summary.md"}

cd "$ROOT_DIR"

mkdir -p "$OUTPUT_DIR"
mkdir -p "$ROOT_DIR/bin"

go build -o ./bin/limier .

./bin/limier run \
  --ecosystem npm \
  --package left-pad \
  --current 1.0.0 \
  --candidate 1.1.0 \
  --fixture fixtures/npm-app \
  --scenario scenarios/npm.yml \
  --rules rules/default.yml \
  --report "$REPORT_PATH" \
  --summary "$SUMMARY_PATH" \
  --evidence "$OUTPUT_DIR/evidence"

./bin/limier render \
  --format "$RENDER_FORMAT" \
  --input "$REPORT_PATH" \
  --output "$RENDER_OUTPUT"
