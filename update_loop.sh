#!/bin/bash

while true
do
  echo "Running updatedb.py..."
  python3 /Users/jnewlin/src/venvs/fshub-tools/updatedb.py
  echo "Sleeping for 5 minutes..."
  sleep 300
done
