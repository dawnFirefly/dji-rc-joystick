package windows

import (
	"fyne.io/fyne/v2/container"
)

// // // // // // // // // //

func (obj *Obj) tabItems() []*container.TabItem {
	return []*container.TabItem{
		container.NewTabItem(obj.lg("tab.home", "Home"), obj.renderHome()),
		container.NewTabItem(obj.lg("tab.ui", "Interface"), obj.renderUI()),
		container.NewTabItem(obj.lg("tab.about", "About"), obj.renderAbout()),
	}
}

func (obj *Obj) newTabs() *container.AppTabs {
	tabs := container.NewAppTabs(obj.tabItems()...)
	tabs.SetTabLocation(container.TabLocationTop)
	tabs.SelectIndex(0)
	tabs.OnSelected = func(item *container.TabItem) {
		obj.log.Debug().
			Str("tab", item.Text).
			Int("index", tabs.SelectedIndex()).
			Msg("GUI tab selected")
	}
	return tabs
}

func (obj *Obj) refreshTabs(selected int) {
	if selected < 0 || selected >= len(obj.tabs.Items) {
		selected = 0
	}

	obj.tabs.SetItems(obj.tabItems())
	obj.tabs.SetTabLocation(container.TabLocationTop)
	obj.tabs.SelectIndex(selected)
	obj.tabs.Refresh()
}
