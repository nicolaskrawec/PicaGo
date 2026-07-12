#!/usr/bin/env bash

set -euo pipefail

version=""
targets="windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64"
output_dir="dist"
goamd64="v1"

usage() {
    cat <<'EOF'
Usage: ./scripts/build.sh [options]

Options:
  --version VERSION       Override the version derived from Git
  --targets TARGETS       Comma- or space-separated GOOS/GOARCH targets
  --output-dir DIRECTORY  Artifact directory relative to the project root
  --goamd64 LEVEL         GOAMD64 level: v1, v2, v3 or v4 (default: v1)
  -h, --help              Show this help
EOF
}

while (($#)); do
    case "$1" in
        --version)
            version="${2:?missing value for --version}"
            shift 2
            ;;
        --targets)
            targets="${2:?missing value for --targets}"
            shift 2
            ;;
        --output-dir)
            output_dir="${2:?missing value for --output-dir}"
            shift 2
            ;;
        --goamd64)
            goamd64="${2:?missing value for --goamd64}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

case "$goamd64" in
    v1|v2|v3|v4) ;;
    *) echo "Invalid --goamd64 value: $goamd64" >&2; exit 2 ;;
esac

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
project_root="$(cd -- "$script_dir/.." && pwd)"
cd "$project_root"

build_version() {
    if [[ -n "$version" ]]; then
        printf '%s\n' "$version"
        return
    fi

    local sha tag exact count resolved dirty=""
    sha="$(git rev-parse --short HEAD 2>/dev/null || true)"
    if [[ -z "$sha" ]]; then
        date '+dev-%Y%m%d%H%M%S'
        return
    fi

    tag="$(git tag --list 'v*' --sort=-creatordate | sed -n '1p')"
    exact="$(git tag --points-at HEAD --list 'v*' | sed -n '1p')"
    if ! git diff --quiet --ignore-submodules HEAD --; then
        dirty=".dirty"
    fi

    if [[ -n "$exact" ]]; then
        resolved="$exact"
    elif [[ -n "$tag" ]]; then
        count="$(git rev-list "$tag..HEAD" --count 2>/dev/null || printf '0')"
        resolved="$tag+$count.$sha"
    else
        resolved="v0.0.0+$sha"
    fi
    printf '%s%s\n' "$resolved" "$dirty"
}

rsrc_tool() {
    local tool
    tool="$(go env GOPATH)/bin/rsrc"
    if [[ ! -x "$tool" ]]; then
        echo "rsrc not found. Install it with: go install github.com/akavel/rsrc@latest" >&2
        return 1
    fi
    printf '%s\n' "$tool"
}

generated_resource=""
cleanup() {
    if [[ -n "$generated_resource" ]]; then
        rm -f -- "$generated_resource"
    fi
}
trap cleanup EXIT

resolved_version="$(build_version)"
mkdir -p -- "$project_root/$output_dir"
targets="${targets//,/ }"

echo "Building version $resolved_version"
for target in $targets; do
    if [[ "$target" != */* || "$target" == */*/* ]]; then
        echo "Invalid target '$target'. Expected format goos/goarch." >&2
        exit 2
    fi
    goos="${target%%/*}"
    goarch="${target##*/}"
    suffix=""
    ldflags="-s -w -X viewergo/internal/app.Version=$resolved_version"

    if [[ "$goos" == "windows" ]]; then
        suffix=".exe"
        ldflags="$ldflags -H windowsgui"
        generated_resource="$project_root/rsrc_windows_${goarch}.syso"
        "$(rsrc_tool)" -ico "$project_root/internal/assets/icon.ico" -arch "$goarch" -o "$generated_resource"
    fi

    artifact="PicaGo_${resolved_version}_${goos}_${goarch}${suffix}"
    echo " -> $target"
    if [[ "$goarch" == "amd64" ]]; then
        GOOS="$goos" GOARCH="$goarch" GOAMD64="$goamd64" CGO_ENABLED=0 \
            go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$project_root/$output_dir/$artifact" .
    else
        GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
            go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$project_root/$output_dir/$artifact" .
    fi

    cleanup
    generated_resource=""
done

echo "Artifacts written to $output_dir"
