package compute

import (
	"log"

	"github.com/gbl08ma/sqalx"
)

var rootSqalxNode sqalx.Node
var mainLog *log.Logger

// Initialize initializes the package
func Initialize(snode sqalx.Node, log *log.Logger) {
	rootSqalxNode = snode
	mainLog = log
}
