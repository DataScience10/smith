package app

import "github.com/atlassian/smith"

type Processor interface {
	Rebuild(*smith.Template)
	RebuildByName(namespace, name string)
}
