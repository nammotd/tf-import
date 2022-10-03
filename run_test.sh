#!/bin/bash

./tf-import --working-dir="./tests" --addr-file="migration.txt" --terraform-version="1.1.6" --saved-file="imported.txt"
