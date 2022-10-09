package main

import (
  "context"
  "log"
  "github.com/hashicorp/go-version"
  "github.com/hashicorp/hc-install/product"
  "github.com/hashicorp/hc-install/releases"
  "github.com/hashicorp/terraform-exec/tfexec"
  "bufio"
  "os"
  "strings"
  "github.com/fatih/color"
  "regexp"
  "encoding/json"
  "io/ioutil"
  "fmt"
  "path/filepath"
)
type State struct {
  Version interface{} `json:"version"`
  TerraformVersion interface{} `json:"terraform_version"`
  Serial interface{} `json:"serial"`
  Lineage interface{} `json:"lineage"`
  Outputs interface{} `json:"outputs"`
  Resources interface{} `json:"resources"`
}

func writeState(x []string, stateFile string, workingDir string) {
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
    TerraformVersion: "1.1.7",
    Serial: 1,
    Lineage: "47020475-021a-182f-eea6-1080efaa8a2c",
    Outputs: outputs,
    Resources: resources,
  }
  value, err := json.Marshal(state)
  if err != nil {
    fmt.Println(err)
  }
  _ = ioutil.WriteFile(fileName, value, 0644)
}

func execImport(tf *tfexec.Terraform, workingDir string, stateFile string, resources []string, ch chan []string) {
  var result []string
  for _,resource := range resources {
    r := strings.Split(resource, " ")
    rAddr := &r[0]
    rId := &r[1]

    prefix := "import_"
    suffix := ".tfstate"
    stateOutFile := prefix + *rAddr + suffix
    bold := color.New(color.Bold).SprintFunc()
    plan := tf.Import(context.Background(), *rAddr, *rId,
        tfexec.StateOut(stateOutFile),
        tfexec.State(stateFile),
        tfexec.Lock(false),
      )
    if plan == nil {
      color.Green("[+] %s => IMPORTED", bold(resource))
      result = append(result, filepath.Join(workingDir, stateOutFile))
    } else {
      pattern := regexp.MustCompile(`Error: (.*)`)
      matches := pattern.FindStringSubmatch(plan.Error())
      cause := &matches[1]
      switch *cause {
      case "Resource already managed by Terraform":
        color.Blue("[+] %s => IGNORED | %s already managed by Terraform", bold(resource), bold(*rAddr))
      case "Cannot import non-existent remote object":
        color.Blue("[+] %s => IGNORED | %s does not exist", bold(resource), bold(*rId))
      default:
        color.Red("[+] %s => FAILED | %s", bold(resource), *cause)
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

func main() {
  installer := &releases.ExactVersion{
    Product: product.Terraform,
    Version: version.Must(version.NewVersion("1.1.7")),
  }
  execPath, err := installer.Install(context.Background())
  if err != nil {
    log.Fatalf("error installing Terraform: %s", err)
  }
  workingDir := "/Users/namnguyen/lab/go_learning/import/work"
  tf, err := tfexec.NewTerraform(workingDir, execPath)
  if err != nil {
    log.Fatalf("error running NewTerraform: %s", err)
  }
  err = tf.Init(context.Background())
  if err != nil {
    log.Fatalf("error running Init: %s", err)
  }

  file, err := os.Open("work/migration.txt")
  if err != nil {
      log.Fatal(err)
  }
  defer file.Close()

  r := bufio.NewScanner(file)
  resources := []string{}
  for r.Scan() {
    resources = append(resources, r.Text())
  }
  resources = resources[:20]

  stateFile := "terraform.tfstate"
  max := 6
  chunkSize := len(resources)/max
  remainer := len(resources)%chunkSize
  if remainer != 0 {
    max += 1
  }
  ch := make(chan []string, chunkSize)
  for x := 0; x < max; x++ {
    start := x*chunkSize
    end := chunkSize*(x+1)

    if end > len(resources) {
      end = len(resources)
    }
    go execImport(tf, workingDir, stateFile, resources[start:end], ch)
  }
  result := []string{}
  for x := 0; x < max; x++ {
    result = append(result, <-ch...)
  }
  writeState(result, stateFile, workingDir)
  cleanUp(result)
}
