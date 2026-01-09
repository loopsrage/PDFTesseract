package searcher

import "Tesseract/lib/jsonContainers/container"

type ContainerSearcher interface {
	Current(container *container.Container)
}
