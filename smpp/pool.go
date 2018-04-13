// pool.go
package smpp

import (
	//"fmt"

	"github.com/ivpusic/grpool"
)

var defaultpool *grpool.Pool

func InitPool(numw int, numj int) {

	defaultpool = grpool.NewPool(numw, numj)

}

func Queue(f func()) {
	defaultpool.JobQueue <- f
}

func ReleasePool() {
	defaultpool.Release()
	defaultpool.WaitAll()
}
