#!/bin/bash

# Load COPILOT_API_KEY from .env
if [ -f .env ]; then
    export $(grep COPILOT_API_KEY .env | xargs)
fi

if [ -z "$COPILOT_API_KEY" ]; then
    echo "Error: COPILOT_API_KEY not found in .env"
    exit 1
fi

echo "Fetching available models from GitHub Copilot..."
echo "------------------------------------------------"

response=$(curl -s -H "Authorization: Bearer $COPILOT_API_KEY" \
     -H "Editor-Version: vscode/1.85.1" \
     -H "Editor-Plugin-Version: copilot/1.143.0" \
     https://api.githubcopilot.com/models)

# Check if jq is installed for pretty printing
if command -v jq &> /dev/null; then
    echo "$response" | jq -r '.data[] | "\(.id) (Vendor: \(.vendor), Family: \(.capabilities.family))"'
else
    echo "$response"
    echo ""
    echo "Tip: Install 'jq' for better formatting."
fi
