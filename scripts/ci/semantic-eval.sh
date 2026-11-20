#!/usr/bin/env bash
set -euo pipefail
. "$(dirname "$0")/common.sh"

changed=0
while IFS= read -r file; do
  if is_prompt_or_model_path "$file"; then
    echo "prompt or model path changed: $file"
    changed=1
  fi
done < <(changed_files)

if [[ "$changed" -eq 0 ]]; then
  echo "semantic-eval: no prompt or model-configuration paths changed; skipping replay."
  exit 0
fi

echo "semantic-eval: replaying frozen Owner B evaluation set"
(cd test/offline/eval-b && go test ./replay -count=1)
echo "semantic-eval: replay passed"
