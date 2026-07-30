package language

import "testing"

// // // // // // // // // //

func TestNewAndSet(t *testing.T) {
	obj, err := New("en")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if obj.Current() != "en" {
		t.Fatalf("Current() = %q, want en", obj.Current())
	}

	if err = obj.Set("ru-RU"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if obj.Current() != "ru-RU" {
		t.Fatalf("Current() = %q, want ru-RU", obj.Current())
	}
	if err = obj.Set("unknown"); err == nil {
		t.Fatal("Set() accepted an unknown language")
	}
}

func TestGetFallsBackToEnglishAndDefault(t *testing.T) {
	obj := &Obj{current: "unsupported"}

	if value := obj.Get("home.mode.automatic", "default"); value != "Automatic" {
		t.Fatalf("Get() = %q, want English fallback", value)
	}
	if value := obj.Get("missing", "default"); value != "default" {
		t.Fatalf("Get() = %q, want default", value)
	}
}

func TestSearchActionTranslations(t *testing.T) {
	tests := map[string]string{
		"en":    "Stop searching and continue",
		"ru-RU": "Остановить поиск и продолжить",
	}
	for locale, want := range tests {
		obj, err := New(locale)
		if err != nil {
			t.Fatalf("New(%q) error = %v", locale, err)
		}
		if got := obj.Get("home.search.accept_found", "missing"); got != want {
			t.Fatalf("Get(%q) = %q, want %q", locale, got, want)
		}
	}
}

func TestControlTranslations(t *testing.T) {
	tests := map[string]map[string]string{
		"en": {
			"home.controls.left_stick":         "Left stick",
			"home.controls.button.camera_half": "Half-press",
		},
		"ru-RU": {
			"home.controls.left_stick":         "Левый стик",
			"home.controls.button.camera_half": "Полунажатие",
		},
	}
	for locale, translations := range tests {
		obj, err := New(locale)
		if err != nil {
			t.Fatalf("New(%q) error = %v", locale, err)
		}
		for key, want := range translations {
			if got := obj.Get(key, "missing"); got != want {
				t.Fatalf(
					"Get(%q, %q) = %q, want %q",
					locale,
					key,
					got,
					want,
				)
			}
		}
	}
}

func TestGamepadTranslations(t *testing.T) {
	tests := map[string]map[string]string{
		"en": {
			"home.gamepad.save":           "Save settings",
			"home.gamepad.status.waiting": "Waiting for remote — controls are neutral",
		},
		"ru-RU": {
			"home.gamepad.save":           "Сохранить настройки",
			"home.gamepad.status.waiting": "Ожидание пульта — элементы в нейтральном положении",
		},
	}
	for locale, translations := range tests {
		obj, err := New(locale)
		if err != nil {
			t.Fatalf("New(%q) error = %v", locale, err)
		}
		for key, want := range translations {
			if got := obj.Get(key, "missing"); got != want {
				t.Fatalf(
					"Get(%q, %q) = %q, want %q",
					locale,
					key,
					got,
					want,
				)
			}
		}
	}
}

func TestListUsesGeneratedLanguageNames(t *testing.T) {
	obj, err := New("en")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	options := obj.List()
	if len(options) != 2 {
		t.Fatalf("List() returned %d options, want 2", len(options))
	}

	for index, option := range options {
		if index > 0 && options[index-1].Key >= option.Key {
			t.Fatalf("List() is not sorted: %q before %q", options[index-1].Key, option.Key)
		}
	}
}
