#!/usr/bin/env sh
set -e
MODEL="${1:-llama3.2}"
echo "Pulling Ollama model: $MODEL"
ollama pull "$MODEL"
