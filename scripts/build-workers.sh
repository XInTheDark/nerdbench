#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
host_goos="$(go env GOOS)"
host_goarch="$(go env GOARCH)"
target="${NERDBENCH_TARGET:-$host_goos-$host_goarch}"
target_os="${target%-*}"
target_arch="${target##*-}"
out_dir="$repo_root/build/workers/$target"
embed_dir="$repo_root/internal/assets/embedded"
third_party="$repo_root/third_party/src"

case "$target_os-$target_arch" in
  linux-amd64|linux-arm64|darwin-arm64) ;;
  *) echo "unsupported NERDBENCH_TARGET: $target" >&2; exit 1 ;;
esac

if [ "$target_os" != "$host_goos" ] || [ "$target_arch" != "$host_goarch" ]; then
  echo "build-workers.sh requires a native target environment; host is $host_goos-$host_goarch but target is $target" >&2
  echo "Use a $target container/runner instead of producing host-arch workers with target names." >&2
  exit 1
fi

if [ ! -d "$third_party/c-ray/.git" ] || [ ! -d "$third_party/sqlite/.git" ] || [ ! -d "$third_party/tinycc/.git" ]; then
  "$repo_root/scripts/fetch-third-party.sh"
fi

mkdir -p "$out_dir" "$embed_dir"

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

install_worker() {
  src="$1"
  dst="$2"
  if [ "$src" != "$dst" ]; then
    cp "$src" "$dst"
  fi
  if [ "$target_os" = "linux" ] && command -v file >/dev/null 2>&1; then
    desc="$(file "$dst")"
    case "$desc" in
      *"dynamically linked"*)
        echo "worker is dynamically linked and cannot be embedded as zero-dependency: $dst" >&2
        echo "$desc" >&2
        exit 1
        ;;
    esac
  fi
  cp "$dst" "$embed_dir/$(basename "$dst")"
}

compiler="$(${CC:-cc} --version 2>/dev/null | head -n 1 | sed 's/"/\\"/g')"
var_suffix="$(printf '%s' "$target_os$target_arch" | tr -cd '[:alnum:]')"
static_ldflags=""
sysbench_extra_ldflags=""
c_ray_static=""
zstd_system_libs="HAVE_ZLIB=0 HAVE_LZMA=0 HAVE_LZ4=0"
ggml_libs="-lggml -lggml-base -lggml-cpu -lc++"
if [ "$target_os" = "linux" ]; then
  static_ldflags="-static"
  sysbench_extra_ldflags="--with-extra-ldflags=-all-static"
  c_ray_static="STATIC=1"
  ggml_libs="-Wl,--start-group -lggml -lggml-base -lggml-cpu -Wl,--end-group -ldl"
else
  zstd_system_libs=""
fi

# ─── C-Ray ───────────────────────────────────────────────────────────
build_c_ray() {
  src="$third_party/c-ray"
  make -C "$src" clean >/dev/null 2>&1 || true
  make -C "$src" -j "${JOBS:-2}" OPT="${OPT:--O2}" $c_ray_static >/dev/null
  install_worker "$src/bin/c-ray" "$out_dir/c-ray-$target"
}

# ─── SQLite Speedtest ────────────────────────────────────────────────
build_sqlite() {
  src="$third_party/sqlite"
  (cd "$src" && ./configure --disable-tcl >/dev/null && make -j "${JOBS:-2}" sqlite3.c >/dev/null)
  ${CC:-cc} -O2 \
    -DSQLITE_THREADSAFE=0 \
    -DSQLITE_OMIT_LOAD_EXTENSION \
    -DSQLITE_TEMP_STORE=3 \
    -I"$src" \
    "$src/sqlite3.c" "$src/test/speedtest1.c" \
    $static_ldflags \
    -lpthread -ldl -lm \
    -o "$out_dir/sqlite-speedtest-$target"
  install_worker "$out_dir/sqlite-speedtest-$target" "$out_dir/sqlite-speedtest-$target"
}

# ─── sysbench ────────────────────────────────────────────────────────
build_sysbench() {
  src="$third_party/sysbench"
  if [ ! -f "$src/configure" ]; then
    (cd "$src" && ./autogen.sh >/dev/null 2>&1)
  fi
  (cd "$src" && make clean >/dev/null 2>&1 || true)
  # Use system LuaJIT on macOS, bundled on Linux
  case "$(uname -s)" in
    Darwin)
      (cd "$src" && ./configure --without-mysql --without-pgsql --without-redis --with-system-luajit >/dev/null 2>&1)
      ;;
    *)
      (cd "$src" && ./configure --without-mysql --without-pgsql --without-redis $sysbench_extra_ldflags >/dev/null 2>&1)
      ;;
  esac
  (cd "$src" && make -j "${JOBS:-2}" >/dev/null 2>&1)
  # sysbench binary is in src/ subdirectory
  if [ -f "$src/src/sysbench" ]; then
    install_worker "$src/src/sysbench" "$out_dir/sysbench-$target"
  else
    install_worker "$src/sysbench" "$out_dir/sysbench-$target"
  fi
}

# ─── Stockfish ───────────────────────────────────────────────────────
build_stockfish() {
  src="$third_party/stockfish"
  # Stockfish Makefile is in src/ subdirectory
  make_dir="$src/src"
  if [ ! -f "$make_dir/Makefile" ]; then
    make_dir="$src"
  fi
  case "$target_os-$target_arch" in
    darwin-arm64)
      make -C "$make_dir" -j "${JOBS:-2}" build >/dev/null 2>&1
      ;;
    linux-amd64)
      make -C "$make_dir" -j "${JOBS:-2}" build ARCH=x86-64-ssse3 EXTRALDFLAGS="$static_ldflags" >/dev/null 2>&1 || \
      make -C "$make_dir" -j "${JOBS:-2}" build EXTRALDFLAGS="$static_ldflags" >/dev/null 2>&1
      ;;
    linux-arm64)
      make -C "$make_dir" -j "${JOBS:-2}" build ARCH=armv8 EXTRALDFLAGS="$static_ldflags" >/dev/null 2>&1 || \
      make -C "$make_dir" -j "${JOBS:-2}" build EXTRALDFLAGS="$static_ldflags" >/dev/null 2>&1
      ;;
    *)
      make -C "$make_dir" -j "${JOBS:-2}" build >/dev/null 2>&1
      ;;
  esac
  install_worker "$make_dir/stockfish" "$out_dir/stockfish-$target"
}

# ─── OpenSSL Speed ───────────────────────────────────────────────────
build_openssl() {
  src="$third_party/openssl"
  if [ ! -f "$src/Makefile" ]; then
    (cd "$src" && ./Configure --prefix="$src/install" --openssldir="$src/install" \
      no-shared no-dso no-engine no-tests no-ssl3 no-comp $static_ldflags 2>/dev/null)
  fi
  (cd "$src" && make -j "${JOBS:-2}" >/dev/null 2>&1)
  install_worker "$src/apps/openssl" "$out_dir/openssl-speed-$target"
}

# ─── zstd ────────────────────────────────────────────────────────────
build_zstd() {
  src="$third_party/zstd"
  make -C "$src/lib" -j "${JOBS:-2}" libzstd.a 2>/dev/null
  make -C "$src/programs" -j "${JOBS:-2}" zstd LDFLAGS="$static_ldflags" $zstd_system_libs 2>/dev/null
  install_worker "$src/programs/zstd" "$out_dir/zstd-$target"
}

# ─── ggml ML Kernel ─────────────────────────────────────────────────
build_ggml() {
  src="$third_party/llama.cpp"
  mkdir -p "$src/build"
  cmake -S "$src" -B "$src/build" \
    -DCMAKE_C_FLAGS="-O2" \
    -DCMAKE_BUILD_TYPE=Release \
    -DGGML_NATIVE=OFF \
    -DGGML_CPU=ON \
    -DGGML_AVX=OFF \
    -DGGML_AVX2=OFF \
    -DGGML_F16C=OFF \
    -DGGML_FMA=OFF \
    -DGGML_LLAMAFILE=OFF \
    -DGGML_OPENMP=OFF \
    -DLLAMA_BUILD_EXAMPLES=OFF \
    -DLLAMA_BUILD_TESTS=OFF \
    -DLLAMA_BUILD_SERVER=OFF \
    -DBUILD_SHARED_LIBS=OFF \
    >/dev/null 2>&1
  cmake --build "$src/build" -j "${JOBS:-2}" --target ggml ggml-base ggml-cpu

  # Write the ggml benchmark worker
  cat > "$out_dir/ggml-bench.c" <<'CEOF'
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "ggml.h"
#include "ggml-cpu.h"

static double now_sec(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec * 1e-9;
}

int main(int argc, char **argv) {
    int n_threads = 1;
    int n_iter = 10;
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--threads") == 0 && i + 1 < argc) n_threads = atoi(argv[++i]);
        if (strcmp(argv[i], "--iter") == 0 && i + 1 < argc) n_iter = atoi(argv[++i]);
    }

    size_t buf_size = 256 * 1024 * 1024;
    void *buf = malloc(buf_size);
    struct ggml_init_params params = { buf_size, buf, false };
    struct ggml_context *ctx = ggml_init(params);
    if (!ctx) { fprintf(stderr, "ggml_init failed\n"); free(buf); return 1; }

    int M = 512, N = 512, K = 512;
    struct ggml_tensor *A = ggml_new_tensor_2d(ctx, GGML_TYPE_F32, K, M);
    struct ggml_tensor *B = ggml_new_tensor_2d(ctx, GGML_TYPE_F32, N, K);
    struct ggml_tensor *C = ggml_mul_mat(ctx, A, B);

    float *data_a = (float *)A->data;
    float *data_b = (float *)B->data;
    for (int i = 0; i < M * K; i++) data_a[i] = (float)(i % 1000) * 0.001f;
    for (int i = 0; i < K * N; i++) data_b[i] = (float)(i % 1000) * 0.001f;

    struct ggml_cgraph *gf = ggml_new_graph(ctx);
    ggml_build_forward_expand(gf, C);

    double start = now_sec();
    for (int i = 0; i < n_iter; i++) {
        ggml_graph_compute_with_ctx(ctx, gf, n_threads);
    }
    double elapsed = now_sec() - start;

    double flops = 2.0 * M * N * K * n_iter;
    double gflops = flops / elapsed / 1e9;

    printf("NERDBENCH_GGML_RESULT:%.6f\n", gflops);
    printf("ggml %dx%dx%d matmul x%d iter: %.3f GFLOP/s\n", M, N, K, n_iter, gflops);

    ggml_free(ctx);
    free(buf);
    return 0;
}
CEOF

  # Link with g++ because ggml-cpu is C++
  ggml_inc="$src/ggml/include"
  ${CXX:-g++} -O2 -std=c++17 \
    -I"$ggml_inc" -I"$src/include" -I"$src/src" \
    "$out_dir/ggml-bench.c" \
    -L"$src/build/ggml/src" $ggml_libs \
    $static_ldflags \
    -lpthread -lm \
    -o "$out_dir/ggml-ml-kernel-$target" || {
    echo "warning: ggml worker build failed" >&2
    return 1
  }
  install_worker "$out_dir/ggml-ml-kernel-$target" "$out_dir/ggml-ml-kernel-$target"
}

# ─── TinyCC Compile ──────────────────────────────────────────────────
build_tinycc() {
  src="$third_party/tinycc"
  (cd "$src" && ./configure --prefix="$(pwd)/install" --extra-ldflags="$static_ldflags" >/dev/null 2>&1)
  (cd "$src" && make -j "${JOBS:-2}" tcc)
  install_worker "$src/tcc" "$out_dir/tinycc-compile-$target"
}

# ─── Build all workers ───────────────────────────────────────────────
build_c_ray
build_sqlite
build_sysbench
build_stockfish
build_openssl
build_zstd
build_ggml
build_tinycc

# ─── Generate embedded asset registration ────────────────────────────
# Collect all workers and generate Go embed code
workers=""
init_body=""

for f in "$embed_dir"/*-"$target"; do
  [ -f "$f" ] || continue
  bn="$(basename "$f")"
  case "$bn" in
    c-ray-*)            var="cRay"; bench="c-ray"; src_url="https://github.com/vkoskiv/c-ray"; lic="MIT"; cmd="c-ray scene.json"; bf="make OPT=${OPT:--O2}" ;;
    sqlite-speedtest-*) var="sqliteSpeedtest"; bench="sqlite-speedtest"; src_url="https://github.com/sqlite/sqlite"; lic="Public Domain"; cmd="sqlite-speedtest --memdb --size N --repeat 1 --testset main"; bf="-O2 -DSQLITE_THREADSAFE=0" ;;
    sysbench-*)         var="sysbenchWorker"; bench="sysbench"; src_url="https://github.com/akopytov/sysbench"; lic="GPL-2.0-or-later"; cmd="sysbench cpu --threads=N run"; bf="--without-mysql --without-pgsql" ;;
    stockfish-*)        var="stockfishWorker"; bench="stockfish"; src_url="https://github.com/official-stockfish/Stockfish"; lic="GPL-3.0"; cmd="stockfish bench"; bf="build ARCH=default" ;;
    openssl-speed-*)    var="opensslSpeed"; bench="openssl-speed"; src_url="https://github.com/openssl/openssl"; lic="Apache-2.0"; cmd="openssl speed"; bf="no-shared no-dso no-engine" ;;
    zstd-*)             var="zstdWorker"; bench="zstd"; src_url="https://github.com/facebook/zstd"; lic="BSD-3-Clause"; cmd="zstd -b1"; bf="libzstd.a" ;;
    ggml-ml-kernel-*)   var="ggmlMlKernel"; bench="ggml-ml-kernel"; src_url="https://github.com/ggml-org/llama.cpp"; lic="MIT"; cmd="ggml-ml-kernel --threads N"; bf="-O2 -lggml -lggml-cpu" ;;
    tinycc-compile-*)   var="tinyccCompile"; bench="tinycc-compile"; src_url="https://github.com/TinyCC/tinycc"; lic="LGPL-2.1"; cmd="tinycc-compile -c corpus.c"; bf="native tcc binary" ;;
    *) continue ;;
  esac

  var_name="${var}${var_suffix}"
  # Get revision from the corresponding git repo
  case "$bench" in
    c-ray)           rev="$(git -C "$third_party/c-ray" rev-parse HEAD 2>/dev/null || echo pending)" ;;
    sqlite-speedtest) rev="$(git -C "$third_party/sqlite" rev-parse HEAD 2>/dev/null || echo pending)" ;;
    sysbench)        rev="$(git -C "$third_party/sysbench" rev-parse HEAD 2>/dev/null || echo pending)" ;;
    stockfish)       rev="$(git -C "$third_party/stockfish" rev-parse HEAD 2>/dev/null || echo pending)" ;;
    openssl-speed)   rev="$(git -C "$third_party/openssl" rev-parse HEAD 2>/dev/null || echo pending)" ;;
    zstd)            rev="$(git -C "$third_party/zstd" rev-parse HEAD 2>/dev/null || echo pending)" ;;
    ggml-ml-kernel)  rev="$(git -C "$third_party/llama.cpp" rev-parse HEAD 2>/dev/null || echo pending)" ;;
    tinycc-compile)  rev="$(git -C "$third_party/tinycc" rev-parse HEAD 2>/dev/null || echo pending)" ;;
  esac

  workers="${workers}//go:embed embedded/${bn}
var ${var_name} []byte

"

  init_body="${init_body}
	generatedWorkers = append(generatedWorkers, WorkerAsset{
		Name:       \"${bn}\",
		Benchmark:  \"${bench}\",
		OS:         \"${target_os}\",
		Arch:       \"${target_arch}\",
		SHA256:     \"$(sha256 "$embed_dir/$bn")\",
		Bytes:      ${var_name},
		Source:     \"${src_url}\",
		Revision:   \"${rev}\",
		License:    \"${lic}\",
		Compiler:   \"${compiler}\",
		BuildFlags: \"${bf}\",
		Command:    \"${cmd}\",
	})
"
done

cat > "$repo_root/internal/assets/generated_workers.go" <<GOF
package assets

import _ "embed"
${workers}
func init() {${init_body}
}
GOF

gofmt -w "$repo_root/internal/assets/generated_workers.go"

echo "built workers for $target -> $embed_dir" >&2
