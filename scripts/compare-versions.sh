#!/bin/bash

# Check if at least 3 arguments are provided
if [ $# -lt 3 ]; then
    echo "Usage: $0 <version1> <version2> <file1> [file2] ..."
    echo "Example: $0 v1.0.49 v1.0.50 config.yaml settings.json"
    exit 1
fi

VERSION1=$1
VERSION2=$2
shift 2

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Process each file
for FILE in "$@"; do
    # Check if file exists
    if [ ! -f "$FILE" ]; then
        echo -e "${RED}SKIP${NC} $FILE (file not found)"
        continue
    fi
    
    # Run both versions and capture output
    OUTPUT1=$(go run github.com/gopatchy/bkl/cmd/bkl@$VERSION1 "$FILE" 2>&1)
    EXIT1=$?
    OUTPUT2=$(go run github.com/gopatchy/bkl/cmd/bkl@$VERSION2 "$FILE" 2>&1)
    EXIT2=$?
    
    # Check if both versions failed with the same error
    if [ $EXIT1 -ne 0 ] && [ $EXIT2 -ne 0 ]; then
        if [ "$OUTPUT1" = "$OUTPUT2" ]; then
            echo -e "${GREEN}PASS${NC} $FILE (both versions error identically)"
        else
            echo -e "${RED}ERROR${NC} $FILE (different errors)"
            echo "  $VERSION1: $OUTPUT1"
            echo "  $VERSION2: $OUTPUT2"
        fi
        continue
    fi
    
    # Check if only one version failed
    if [ $EXIT1 -ne 0 ] || [ $EXIT2 -ne 0 ]; then
        echo -e "${RED}ERROR${NC} $FILE (only one version failed)"
        if [ $EXIT1 -ne 0 ]; then
            echo "  $VERSION1 failed: $OUTPUT1"
        else
            echo "  $VERSION1 succeeded"
        fi
        if [ $EXIT2 -ne 0 ]; then
            echo "  $VERSION2 failed: $OUTPUT2"
        else
            echo "  $VERSION2 succeeded"
        fi
        continue
    fi
    
    # Compare outputs
    if [ "$OUTPUT1" = "$OUTPUT2" ]; then
        echo -e "${GREEN}PASS${NC} $FILE"
    else
        echo -e "${RED}DIFF${NC} $FILE"
        # Use process substitution to diff without temp files
        diff -u <(echo "$OUTPUT1") <(echo "$OUTPUT2") | sed "s/^--- .*/--- $VERSION1/" | sed "s/^+++ .*/+++ $VERSION2/"
        echo
    fi
done