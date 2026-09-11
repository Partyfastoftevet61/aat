#!/bin/sh
# Packages this project's integration kit for the shop API's integrators: the
# files aat-kit.yaml names, with aat-kit.yaml as the kit's aat-project.yaml and
# KIT-README.md as its README.md, and a tarball of the result. internal/,
# layers/, overlays/, visualizers/, and aat-project.yaml are not copied.
#
# Usage: sh package-kit.sh [out-dir]   (default: _output/shop-kit in this project)
#
# Writes <out-dir>/ and <out-dir>.tar.gz. The directory must not exist yet,
# unless it is under this project's _output/, where it is replaced. When the kit
# needs another file, such as an environment include or a docs directory, add it
# to the cp line below: this list and aat-kit.yaml must agree.
set -eu

project="$(cd "$(dirname "$0")" && pwd)"
out="${1:-$project/_output/shop-kit}"
case "$out" in
  /*) ;;
  *) out="$(pwd)/$out" ;;
esac
case "$out" in
  "$project/_output/"*) rm -rf "$out" "$out.tar.gz" ;;
esac

mkdir -p "$(dirname "$out")"
mkdir "$out"
cd "$project"
cp -R graph.yaml openapi.yaml domain.yaml env.yaml templates workflows plans "$out/"
cp aat-kit.yaml "$out/aat-project.yaml"
cp KIT-README.md "$out/README.md"
printf '_output/\nstate.json\n' >"$out/.gitignore"

# COPYFILE_DISABLE keeps macOS tar from adding ._ metadata files.
COPYFILE_DISABLE=1 tar -czf "$out.tar.gz" -C "$(dirname "$out")" "$(basename "$out")"
echo "package-kit: wrote $out/ and $out.tar.gz"
