#!/bin/bash
  JWT=""

  GROUP=$(curl -s -X POST "https://api.pinata.cloud/v3/files/groups" \
    -H "Authorization: Bearer $JWT" \
    -H "Content-Type: application/json" \
    -d '{"name": "NFT Uploads", "is_public": true}')

  echo "$GROUP"
  echo ""
  echo "Group ID: $(echo "$GROUP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)"