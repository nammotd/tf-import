## Introduction

### Limitations
-   Support only AWS resources currently

### How to prepare an address file
- There is an example (migration.txt)[tests/migration.txt]
- Explain:
    -   the file contains 3 components:
        -   the terraform resources address (this needs to be explicit since you design your own modules)
        -   the actual resource id (can be referred from official Terraform pages)
        -   the region of that resource
    -   those components are seperated by an `indicator`, you can have a different indicator (default value is a white space), then specify that indicator in the parameter

### Parameters
- you can get parameters instruction by running `./tf-import --help`

### How to build
- run `go build`

### How to run
- checkout `run_test.sh`
