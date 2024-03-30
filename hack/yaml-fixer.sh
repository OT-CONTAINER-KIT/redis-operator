#!/bin/bash

export YAMLFIX_LINE_LENGTH="150"

# This script is used to fix the yaml files in the repository.
# Note: yamlfix utility doesn't seems to be much mature idk but it is fixing the .crt .tls .ca extenstion present in testdata

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
# run in parallel so stdout is mixed up
if [ -n "$DIR" ]; then
  echo "------------------------------"
  echo "Fixing all YAML files in the directory and its subdirectories: $DIR"
  find "$DIR" \( -name '*.yml' -o -name '*.yaml' \) -type f -print0 | xargs -0 -P 4 -I {} yamlfix "{}"
  echo "yamlfix has been applied to all YAML files in $DIR and its subdirectories."
fi
