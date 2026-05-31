package tui

import "charm.land/bubbles/v2/viewport"

func newViewport(width, height int) viewport.Model {
	return viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
}

func resizeViewport(vp *viewport.Model, width, height int) {
	vp.SetWidth(width)
	vp.SetHeight(height)
}
