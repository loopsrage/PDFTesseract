package clsr

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func SigClosure(signals []os.Signal, after ...io.Closer) {
	go func() {
		x := make(chan os.Signal, 1)

		sigs := []os.Signal{syscall.SIGKILL, syscall.SIGTERM, os.Interrupt}
		if len(signals) > 0 {
			sigs = append(sigs, signals...)
		}
		signal.Notify(x, sigs...)

		<-x
		for _, f := range after {
			fmt.Println(f.Close())
		}
	}()
}

func OrderedSigClosure(signals []os.Signal, after map[int]io.Closer) {
	go func() {
		x := make(chan os.Signal, 1)

		sigs := []os.Signal{syscall.SIGKILL, syscall.SIGTERM, os.Interrupt}
		if len(signals) > 0 {
			sigs = append(sigs, signals...)
		}
		signal.Notify(x, sigs...)

		<-x
		for i, f := range after {
			err := f.Close()
			if err != nil {
				fmt.Println(i, f.Close())
			}
		}
	}()
}
