package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"tangentry/internal/transcription"
)

func TestWhisperMetalFailureRetriesSameModelOnCPUOnce(t *testing.T) {
	python := executable("", "python3")
	if python == "" {
		t.Skip("Python required for explicit process fixture")
	}
	for _, tc := range []struct {
		name, gpuError, cpuError, wantCode string
		cpuWrites, wantCPU                 bool
	}{
		{name: "metal allocation recovery", gpuError: "ggml_backend_metal_buffer_type_alloc_buffer: failed to allocate Metal buffer (out of memory)", cpuWrites: true, wantCPU: true},
		{name: "CPU failure stops", gpuError: "ggml_metal_buffer_init: failed to allocate buffer", cpuError: "out of memory", wantCPU: true, wantCode: "model_memory"},
		{name: "CPU must produce fresh result", gpuError: "ggml_metal_buffer_init: failed to allocate buffer", wantCPU: true, wantCode: "missing_transcript"},
		{name: "other memory failure is not retried", gpuError: "out of memory", wantCode: "model_memory"},
		{name: "GPU success needs no retry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			calls := filepath.Join(root, "calls.txt")
			model := filepath.Join(root, "chosen-model.bin")
			cpuWrites := "False"
			if tc.cpuWrites {
				cpuWrites = "True"
			}
			script := "#!" + python + `
import pathlib,sys
args=sys.argv[1:]
if '-m' not in args: raise SystemExit(0) # Explicit converter fixture.
assert args[args.index('-m')+1] == ` + strconv.Quote(model) + `
cpu='-ng' in args
with open(` + strconv.Quote(calls) + `,'a') as f: f.write('cpu\n' if cpu else 'gpu\n')
out=pathlib.Path(args[args.index('-of')+1]+'.json')
result='{"transcription":[{"offsets":{"from":0,"to":1000},"text":"recognized speech"}]}'
gpu_error=` + strconv.Quote(tc.gpuError) + `
cpu_error=` + strconv.Quote(tc.cpuError) + `
if not cpu:
 out.write_text(result) # Must not survive a failed attempt.
 if gpu_error:
  print(gpu_error,file=sys.stderr)
  raise SystemExit(1)
elif cpu_error:
 print(cpu_error,file=sys.stderr)
 raise SystemExit(1)
elif ` + cpuWrites + `:
 out.write_text(result)
`
			binary := filepath.Join(root, "recognizer.py")
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			engine := Transcriber{FFmpeg: binary}
			track, err := engine.transcribeLocal(context.Background(), transcription.Config{Backend: "whisper.cpp", Binary: binary, Model: model, Language: "en"}, "fixture-audio", root, 60, func(string) {})
			if tc.wantCode == "" {
				if err != nil || len(track.Cues) != 1 || track.Cues[0].Text != "recognized speech" {
					t.Fatal(track, err)
				}
			} else {
				var failure *transcription.Failure
				if !errors.As(err, &failure) || failure.Code != tc.wantCode {
					t.Fatal("wrong failure", err)
				}
			}
			got, err := os.ReadFile(calls)
			want := "gpu\n"
			if tc.wantCPU {
				want += "cpu\n"
			}
			if err != nil || string(got) != want {
				t.Fatal("unexpected recognition attempts", string(got), err)
			}
		})
	}
}
