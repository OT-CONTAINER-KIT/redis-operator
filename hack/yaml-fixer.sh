#!/bin/bash

# This script is used to fix the yaml files in the repository.

DIR=""

if [ -n "$1" ]; then
  DIR="$1"
fi

if which yamlfix >/dev/null; then
    echo "yamlfix version is." "$(yamlfix --version)"
else
    echo "yamlfix is not installed. Please install it using 'pip install yamlfix'"
    exit 0
fi


# Since yamlfix is not able to search in a directory recursively
if [ -n "$DIR" ]; then
  echo ------------------------------
  echo "Fixing all yaml files in the directory: $DIR"
  find "$DIR" -type f -print0 | xargs -0 -I {} yamlfix "{}"
  echo "yamlfix has been applied to all YAML files in $DIR and its subdirectories."
fi
