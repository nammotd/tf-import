#!/bin/bash

./tf-import --working-dir="./tests" --addr-file="migration.txt" --terraform-version="1.1.9" --saved-file="imported.txt" --state "source.tfstate" --parallel 6
