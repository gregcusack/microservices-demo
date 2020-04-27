package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sort"

	"github.com/abiosoft/ishell"
	"go.uber.org/zap"
)

var (
	jaegerAddr string
	faultAddr  string
	kialiAddr string

	zLogger *zap.Logger
	sugar   *zap.SugaredLogger

	jc *JaegerClient
	fc *faultServiceClient
	kc *KialiClient
)

func init() {
	flag.StringVar(&jaegerAddr, "jaeger", "cs1380.cs.brown.edu:5000", "address of jaeger service")
	flag.StringVar(&faultAddr, "fault", "cs1380.cs.brown.edu:5000", "address of fault service")
	flag.StringVar(&kialiAddr, "kiali", "cs1380.cs.brown.edu", "address of kiali service")

	zLogger, _ = zap.NewProduction()
	sugar = zLogger.Sugar()
}

func main() {
	flag.Parse()

	defer zLogger.Sync()

	// Create jaeger service client
	jc = NewJaegerClient(jaegerAddr)

	// Create fault service client
	fc = NewFaultServiceClient(faultAddr)

	// Create kiali service client
	kc = NewKialiClient(kialiAddr)

	// Create client shell
	shell := ishell.New()

	shell.AddCmd(&ishell.Cmd{
		Name: "analyze",
		Func: analyze,
		Help: "analyzes the last experiment results",
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "start",
		Func: start,
		Help: "starts fault injection experiment",
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "experiment",
		Func: experiment,
		Help: "starts a full-fledged experiment",
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "continue",
		Func: continueExperiment,
		Help: "continues a full-fledged experiment",
	})

	// print shell help
	fmt.Print(shell.HelpText())

	// Start shell
	shell.Run()
}

func readDir(path string) (names []string, err error) {
	var dirs []os.FileInfo
	dirs, err = ioutil.ReadDir(path)
	if err != nil {
		return
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].IsDir() != dirs[j].IsDir() {
			return dirs[i].IsDir()
		}
		return dirs[i].Name() > dirs[j].Name()
	})

	for _, d := range dirs {
		names = append(names, d.Name())
	}

	return
}
