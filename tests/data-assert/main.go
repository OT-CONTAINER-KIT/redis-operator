package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

// go run main.go gen-resource-yaml
// go run main.go gen-redis-data --host redis-cluster-0.redis-cluster.default.svc.cluster.local --password 123456 --mode cluster/sentinel
// go run main.go chk-redis-data --host redis-cluster-0.redis-cluster.default.svc.cluster.local --password 123456 --mode cluster/sentinel

const (
	hostFlag = "host"
	passFlag = "password"
	modeFlag = "mode"
	totalKey = 1000
)

var (
	host string
	pass string
	mode string
)

func main() {
	rootCmd := &cobra.Command{
		Use: "data-assert",
	}
	rootCmd.AddCommand(&cobra.Command{
		Use: "gen-resource-yaml",
		Run: genResourceYamlCmd,
	})
	rootCmd.AddCommand(&cobra.Command{
		Use: "gen-redis-data",
		Run: printFlags(genRedisDataCmd),
	})
	rootCmd.AddCommand(&cobra.Command{
		Use: "chk-redis-data",
		Run: printFlags(chkRedisDataCmd),
	})

	// add flags
	rootCmd.PersistentFlags().StringVarP(&host, hostFlag, "H", "", "redis host")
	rootCmd.PersistentFlags().StringVarP(&pass, passFlag, "P", "", "redis password")
	rootCmd.PersistentFlags().StringVarP(&mode, modeFlag, "M", "", "redis mode")

	rootCmd.Execute()
}

type cmdWrapperFunc func(cmd *cobra.Command, args []string)

// printFlags print flags
func printFlags(cmdWrapperFunc cmdWrapperFunc) cmdWrapperFunc {
	return func(cmd *cobra.Command, args []string) {
		fmt.Printf("host: %s, password: %s, mode: %s\n", host, pass, mode)
		cmdWrapperFunc(cmd, args)
	}
}

func genRedisDataCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	var rdb redis.UniversalClient

	// Split host string by comma
	hosts := strings.Split(host, ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}

	switch mode {
	case "cluster":
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    hosts,
			Password: pass,
		})
	case "sentinel":
		rdb = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    "myMaster",
			SentinelAddrs: hosts,
			Password:      pass,
		})
	default:
		fmt.Printf("unsupported redis mode: %s\n", mode)
		return
	}
	defer rdb.Close()

	// Generate and write data
	for i := 0; i < totalKey; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err := rdb.Set(ctx, key, value, 0).Err()
		if err != nil {
			fmt.Printf("failed to set key %s: %v\n", key, err)
			return
		}
	}
	fmt.Printf("[OK] successfully generated %d keys\n", totalKey)
}

// DataError represents data consistency check errors
type DataError struct {
	ExpectedCount int // Expected number of keys
	ActualCount   int // Actual number of keys found
}

func (e *DataError) Error() string {
	return fmt.Sprintf("\nData count mismatch:\n  - Expected: %d keys\n  - Actual: %d keys\n  - Missing: %d keys",
		e.ExpectedCount, e.ActualCount, e.ExpectedCount-e.ActualCount)
}

func chkRedisDataCmd(cmd *cobra.Command, args []string) {
	if err := checkRedisData(); err != nil {
		if dataErr, ok := err.(*DataError); ok {
			fmt.Printf("Data consistency check failed: %s\n", dataErr.Error())
			os.Exit(1)
		}
		fmt.Printf("Error occurred during check: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] Data consistency check passed! All %d keys exist\n", totalKey)
}

func checkRedisData() error {
	ctx := context.Background()
	var rdb redis.UniversalClient

	// Split host string by comma
	hosts := strings.Split(host, ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}

	switch mode {
	case "cluster":
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    hosts,
			Password: pass,
		})
	case "sentinel":
		rdb = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    "myMaster",
			SentinelAddrs: hosts,
			Password:      pass,
		})
	default:
		return fmt.Errorf("unsupported redis mode: %s", mode)
	}
	defer rdb.Close()

	// Count existing keys
	actualCount := 0
	for i := 0; i < totalKey; i++ {
		key := fmt.Sprintf("key-%d", i)
		exists, err := rdb.Exists(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("error checking key %s: %w", key, err)
		}
		if exists == 1 {
			actualCount++
		}
	}

	if actualCount != totalKey {
		return &DataError{
			ExpectedCount: totalKey,
			ActualCount:   actualCount,
		}
	}
	return nil
}

func genResourceYamlCmd(cmd *cobra.Command, args []string) {
	mainGoBytes, err := os.ReadFile("main.go")
	if err != nil {
		panic(err)
	}
	indentedMain := "    " + strings.Join(strings.Split(string(mainGoBytes), "\n"), "\n    ")

	goModBytes, err := os.ReadFile("go.mod")
	if err != nil {
		panic(err)
	}
	goModContent := "    " + strings.Join(strings.Split(string(goModBytes), "\n"), "\n    ")

	goSumBytes, err := os.ReadFile("go.sum")
	if err != nil {
		panic(err)
	}
	goSumContent := "    " + strings.Join(strings.Split(string(goSumBytes), "\n"), "\n    ")

	outFile, err := os.Create("resources.yaml")
	if err != nil {
		panic(err)
	}
	defer outFile.Close()

	err = template.Must(template.ParseFiles("resources.yaml.tmpl")).Execute(outFile, map[string]string{
		"Main":   indentedMain,
		"GoMod":  goModContent,
		"GoSum":  goSumContent,
		"Notice": "## DO NOT MODIFY THIS FILE!!!, IT IS GENERATED FROM resources.yaml.tmpl",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("✅resources.yaml generated")
}
