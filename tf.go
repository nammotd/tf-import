package main

import (
  "context"
  "log"
  "regexp"
  "github.com/hashicorp/go-version"
  "github.com/hashicorp/hc-install/product"
  "github.com/hashicorp/hc-install/releases"
  "github.com/hashicorp/terraform-exec/tfexec"
  "bufio"
  "os"
  "strings"
  "github.com/fatih/color"
)

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

  file, err := os.Open("migration.txt")
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
    plan := tf.Import(context.Background(), *rAddr, *rId)
    if plan == nil {
      color.Green("[+] %s => IMPORTED", bold(readLines.Text()))
    } else {
      pattern := regexp.MustCompile(`Error: (.*)`)
      matches := pattern.FindStringSubmatch(plan.Error())
      cause := &matches[1]
      switch *cause {
      case "Resource already managed by Terraform":
        color.Blue("[+] %s => IGNORED | %s already managed by Terraform", bold(readLines.Text()), bold(*rAddr))
      case "Cannot import non-existent remote object":
        color.Blue("[+] %s => IGNORED | %s does not exist", bold(readLines.Text()), bold(*rId))
      default:
        color.Red("[+] %s => FAILED | %s", bold(readLines.Text()),  *cause)
      }
    }
  }
}
