#!/bin/bash

while true
do
  echo "Running updatedb.py..."
  python3 updatedb.py
  echo "Sleeping for 5 minutes..."
  sleep 300
done
