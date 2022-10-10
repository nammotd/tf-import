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
    "io/ioutil"
    "encoding/json"

	"github.com/fatih/color"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
)

type State struct {
  Version interface{} `json:"version"`
  TerraformVersion interface{} `json:"terraform_version"`
  Serial interface{} `json:"serial"`
  Lineage interface{} `json:"lineage"`
  Outputs interface{} `json:"outputs"`
  Resources interface{} `json:"resources"`
}

type aResource struct {
    rAddr string
    rId string
    region string
}

func (r aResource) Text() string {
    result := fmt.Sprintf("%s %s", r.rAddr, r.rId)
    return result
}

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

func trimCharacters(data string) string {
    triming:
    for {
        last := data[len(data)-1:]
        switch last {
        case "\n", "\t", " ":
            data = strings.TrimSuffix(data, last)
        default:
            break triming
        }
    }
    return data
}

func checkImported(filePath string) map[string]bool {
	State := make(map[string]bool)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDONLY, 0644)
	check(err)
	defer file.Close()

    readLines := bufio.NewScanner(file)
	for readLines.Scan() {
	    if len(readLines.Text()) > 0 {
            key := trimCharacters(readLines.Text())
            State[key] = true
	    }
	}
	return State
}

func writeState(x []string, stateFile string, workingDir string, terraformVersion string) {
  var resources []interface{}
  for _,file := range x {
    ref, ok := extractState(file).([]interface{})
    if ok {
      resources = append(resources, ref[0])
    } else {
      resources = append(resources, ref)
    }
  }
  fileName := filepath.Join(workingDir, stateFile)
  var outputs interface{}
  state := State{
    Version: 4,
    TerraformVersion: terraformVersion,
    Serial: 1,
    Lineage: "",
    Outputs: outputs,
    Resources: resources,
  }
  value, err := json.Marshal(state)
  if err != nil {
    fmt.Println(err)
  }
  _ = ioutil.WriteFile(fileName, value, 0644)
}

func execImport(tf *tfexec.Terraform, workingDir string, stateFile string, savedFilePath string, resources []aResource, ch chan []string) {
  var result []string
  bold := color.New(color.Bold).SprintFunc()

  for _,resource := range resources {
    prefix := "import_"
    suffix := ".tfstate"
    stateOutFile := prefix + resource.rAddr + suffix

    os.Setenv("AWS_DEFAULT_REGION", resource.region)
    plan := tf.Import(context.Background(), resource.rAddr, resource.rId,
        tfexec.StateOut(stateOutFile),
        tfexec.State(stateFile),
        tfexec.Lock(false),
      )
    if plan == nil {
      color.Green("[+] %s => IMPORTED", bold(resource.Text()))
      saveImported(resource.rAddr, savedFilePath)
      result = append(result, filepath.Join(workingDir, stateOutFile))
    } else {
      pattern := regexp.MustCompile(`Error: (.*)`)
      matches := pattern.FindStringSubmatch(plan.Error())
      if len(matches) > 0 {
        cause := &matches[1]
        switch *cause {
        case "Resource already managed by Terraform":
          color.Blue("[+] %s => IGNORED | %s already managed by Terraform", bold(resource.Text()), bold(resource.rAddr))
        case "Cannot import non-existent remote object":
          color.Blue("[+] %s => IGNORED | %s does not exist", bold(resource.Text()), bold(resource.rId))
        default:
          color.Red("[+] %s => FAILED | %s", bold(resource.Text()), *cause)
        }
      }
    }
  }
  ch <- result
}

func extractState(stateFile string) interface{} {
  content, err := ioutil.ReadFile(stateFile)
  if err != nil {
    log.Fatal(err)
  }
  var r map[string]interface{}
  err = json.Unmarshal(content, &r)
  if err != nil {
    log.Fatal("Reading statefile failed, ", err)
  }
  return r["resources"]
}

func cleanUp(stateFiles []string) {
  for _,file := range stateFiles {
    err := os.Remove(file)
    if err != nil {
      fmt.Println(err)
    }
  }
}

var workingDir string
var addrFile string
var savedFile string
var   terraformVersion string
var indicator string
var help bool
var stateFile string
var concur int

func main() {
    flag.StringVar(&workingDir, "working-dir", "", "")
    flag.StringVar(&addrFile, "addr-file", "", "")
    flag.StringVar(&savedFile, "saved-file", "imported.txt", "")
    flag.StringVar(&terraformVersion, "terraform-version", "1.1.6", "")
    flag.StringVar(&indicator, "indicator", " ", "")
    flag.StringVar(&stateFile, "state", "terraform.tfstate", "")
    flag.IntVar(&concur, "parallel", 1, "")
    flag.BoolVar(&help, "help", false, ``)
    flag.Parse()
  
    if help {
      fmt.Println(`
tf-import command supports importing existing resources into Terraform state.

Parameters:
  --working-dir: Required, the directory to run Terraform import
  --addr-file: Required, the addresses file for Terraform to refer
  --indicator: Optaionl, the spliter between elements in address file, default value is a white space
  --saved-file: Optional, the file to saved imported resources, default value is imported.txt
  --terraform-version: Optional, the addresses file for Terraform to refer, default value is 1.1.6, 
  --state: Optional, the Terraform tfstate file, default value is terraform.tfstate
  --parallel: Optional, the number of concurrent import-processes to be executed in the same time, default value is 1
`)
      os.Exit(0)
    }

	addrFilePath := filepath.Join(workingDir, addrFile)
	savedFilePath := filepath.Join(workingDir, savedFile)
	importedResources := checkImported(savedFilePath)
    bold := color.New(color.Bold).SprintFunc()

	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion(terraformVersion)),
	}
    color.White(bold("Installing Terraform version ", terraformVersion))
	execPath, err := installer.Install(context.Background())
	if err != nil {
		log.Fatalf("error installing Terraform: %s", err)
	}
	tf, err := tfexec.NewTerraform(workingDir, execPath)
	if err != nil {
		log.Fatalf("error running NewTerraform: %s", err)
	}

	color.White(bold("Terraform init"))
    err = tf.Init(context.Background(), tfexec.Upgrade(true))
	if err != nil {
		log.Fatalf("error running Init: %s", err)
	}

	migrationFile, err := os.Open(addrFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer migrationFile.Close()

	color.White(bold("Terraform import\n"))
    var resources []aResource
	readLines := bufio.NewScanner(migrationFile)
	for readLines.Scan() {
	    if len(readLines.Text()) > 0 {
            resource := strings.Split(readLines.Text(), indicator)
            if len(resource) != 3 {
                log.Fatalf("Indicator is not properly configured, should only return 3 elements per line")
                os.Exit(1)
            } else {
                rAddr := &resource[0]
                rId := &resource[1]
                region := &resource[2]
                _, ok := importedResources[*rAddr]
                if ok == true {
                    color.Blue("[+] %s => IGNORED | %s already imporetd, managed in %s", bold(readLines.Text()), bold(*rAddr), bold(savedFile))
                } else {
                    resources = append(resources, aResource{rAddr: *rAddr, rId: *rId, region: *region})
                }
            }
	    }
    }

    result := []string{}
    reps := len(resources)/concur
    ch := make(chan []string, reps)
    remainer := len(resources)%reps
    if remainer != 0 {
      reps += 1
    }

    for x := 0; x < reps; x++ {
      start := x*concur
      end := concur*(x+1)

      if end > len(resources) {
        end = len(resources)
      }
      go execImport(tf, workingDir, stateFile, savedFilePath, resources[start:end], ch)
    }
    for x := 0; x < reps; x++ {
      result = append(result, <-ch...)
    }
    writeState(result, stateFile, workingDir, terraformVersion)
    cleanUp(result)
}
