package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"flag"
	"fmt"

	"github.com/fatih/color"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func saveImported(rAddr string, filePath string) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	check(err)
	defer file.Close()
	file.WriteString(rAddr + "\n")
}

func checkImported(filePath string) map[string]bool {
	State := make(map[string]bool)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDONLY, 0644)
	check(err)
	defer file.Close()
	readLines := bufio.NewScanner(file)
	for readLines.Scan() {
		key := strings.TrimSuffix(readLines.Text(), "\n")
		State[key] = true
	}
	return State
}

var workingDir string
var addrFile string
var savedFile string
var terraformVersion string
var help bool

func main() {
    flag.StringVar(&workingDir, "working-dir", "", "")
    flag.StringVar(&addrFile, "addr-file", "", "")
    flag.StringVar(&savedFile, "saved-file", "imported.txt", "")
    flag.StringVar(&terraformVersion, "terraform-version", "1.1.6", "")
    flag.BoolVar(&help, "help", false, ``)
    flag.Parse()
  
    if help {
      fmt.Println(`
tf-import command supports importing existing resources into Terraform state.

Parameters:
  --working-dir: Required, the directory to run Terraform import
  --addr-file: Required, the addresses file for Terraform to refer
  --saved-file: Optional, the file to saved imported resources, default value is imported.txt
  --terraform-version: Optional, the addresses file for Terraform to refer, default value is 1.1.6, 
`)
      os.Exit(0)
    }

	addrFilePath := filepath.Join(workingDir, addrFile)
	savedFilePath := filepath.Join(workingDir, savedFile)
	importedResources := checkImported(savedFilePath)

	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion(terraformVersion)),
	}
	execPath, err := installer.Install(context.Background())
	if err != nil {
		log.Fatalf("error installing Terraform: %s", err)
	}
	tf, err := tfexec.NewTerraform(workingDir, execPath)
	if err != nil {
		log.Fatalf("error running NewTerraform: %s", err)
	}

	file, err := os.Open(addrFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	readLines := bufio.NewScanner(file)
	for readLines.Scan() {
		resource := strings.Split(readLines.Text(), " ")
		if len(resource) != 2 {
			log.Fatalf("Indicator is not properly configured, should only return 2 elements per line")
		}
		rAddr := &resource[0]
		rId := &resource[1]

		bold := color.New(color.Bold).SprintFunc()
		_, ok := importedResources[*rAddr]
		if ok == true {
			color.Blue("[+] %s => IGNORED | %s already managed by Terraform", bold(readLines.Text()), bold(*rAddr))
			continue
		}

		plan := tf.Import(context.Background(), *rAddr, *rId)
		if plan == nil {
			color.Green("[+] %s => IMPORTED", bold(readLines.Text()))
			saveImported(*rAddr, savedFilePath)
		} else {
			pattern := regexp.MustCompile(`Error: (.*)`)
			matches := pattern.FindStringSubmatch(plan.Error())
			cause := &matches[1]
			switch *cause {
			case "Resource already managed by Terraform":
				color.Blue("[+] %s => IGNORED | %s already managed by Terraform", bold(readLines.Text()), bold(*rAddr))
				saveImported(*rAddr, savedFilePath)
			case "Cannot import non-existent remote object":
				color.Blue("[+] %s => IGNORED | %s does not exist", bold(readLines.Text()), bold(*rId))
				saveImported(*rAddr, savedFilePath)
			default:
				color.Red("[+] %s => FAILED | %s", bold(readLines.Text()), *cause)
			}
		}
	}
}
