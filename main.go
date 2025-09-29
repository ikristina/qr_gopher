package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/skip2/go-qrcode"
)

type state int

const (
	askName state = iota
	askSite
	askNotes
	done
)

// model for BubbleTea program state.
type model struct {
	state    state
	input    textinput.Model
	name     string
	site     string
	notes    string
	notesMax int
	err      error
	qrASCII  string // to output QR as askii image.
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Enter your name"
	ti.Focus()
	ti.CharLimit = 64

	return model{
		state:    askName,
		input:    ti,
		notesMax: 250,
	}
}

// Init - entrypoint
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// Update - to handle events from user (key press, timer, window resize etc.)
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			switch m.state {
			case askName:
				m.name = text
				m.state = askSite
				m.input.SetValue("")
				m.input.Placeholder = "Enter site (URL)"
				m.input.CharLimit = 128
				return m, nil

			case askSite:
				m.site = text
				m.state = askNotes
				m.input.SetValue("")
				m.input.Placeholder = "Enter notes (optional)"
				m.input.CharLimit = m.notesMax
				return m, nil

			case askNotes:
				m.notes = text
				m.state = done

				// build content
				content := fmt.Sprintf("Name: %s\nSite: %s\nNotes: %s",
					m.name, m.site, m.notes)

				// generate QR png
				qr, err := qrcode.New(content, qrcode.Highest)
				if err != nil {
					m.err = err
					return m, tea.Quit
				}
				qr.DisableBorder = false
				qrImg := qr.Image(512) // QR code as image

				// load gopher
				gopherFile, err := os.Open("imgs/default.png")
				if err != nil {
					m.err = err
					return m, tea.Quit
				}
				defer gopherFile.Close()

				gopherImg, err := png.Decode(gopherFile)
				if err != nil {
					m.err = err
					return m, tea.Quit
				}

				// resize gopher to 25% of qr code image
				scale := 0.25
				gopherW := int(float64(qrImg.Bounds().Dx()) * scale)
				gopherH := int(float64(qrImg.Bounds().Dy()) * scale)
				resized := resizeImage(gopherImg, gopherW, gopherH)

				// Create circular mask
                mask := image.NewAlpha(resized.Bounds())
                cx, cy := resized.Bounds().Dx()/2, resized.Bounds().Dy()/2
                radius := cx
                for y := 0; y < resized.Bounds().Dy(); y++ {
                    for x := 0; x < resized.Bounds().Dx(); x++ {
                        dx := x - cx
                        dy := y - cy
                        if dx*dx+dy*dy <= radius*radius {
                            mask.SetAlpha(x, y, color.Alpha{A: 255}) // opaque inside circle
                        } else {
                            mask.SetAlpha(x, y, color.Alpha{A: 0}) // transparent outside
                        }
                    }
                }

                circularGopher := image.NewRGBA(resized.Bounds())
				draw.DrawMask(circularGopher, resized.Bounds(), resized, image.Point{}, mask, image.Point{}, draw.Over)

                // Prepare output image
                out := image.NewRGBA(qrImg.Bounds())
                draw.Draw(out, qrImg.Bounds(), qrImg, image.Point{}, draw.Src)

                offset := image.Pt(
                    (qrImg.Bounds().Dx()-resized.Bounds().Dx())/2,
                    (qrImg.Bounds().Dy()-resized.Bounds().Dy())/2,
                )

                // Draw circular white background using mask
                white := image.NewUniform(color.White)
                draw.DrawMask(out, resized.Bounds().Add(offset), white, image.Point{}, mask, image.Point{}, draw.Over)

                // Draw circular gopher on top
                draw.Draw(out, circularGopher.Bounds().Add(offset), circularGopher, image.Point{}, draw.Over)


				// save png
				outFile, err := os.Create("output.png")
				if err != nil {
					m.err = err
					return m, tea.Quit
				}
				defer outFile.Close()
				if err := png.Encode(outFile, out); err != nil {
					m.err = err
					return m, tea.Quit
				}

				return m, tea.Quit
			}

		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View - rendering UI as text.
func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	switch m.state {
	case askName, askSite, askNotes:
		return fmt.Sprintf("%s\n\n%s\n", m.input.Placeholder, m.input.View())
	case done:
		return fmt.Sprintf("QR code generated → output.png\n\n%s\n", m.qrASCII)
	}
	return ""
}

// Simple nearest-neighbor resize
func resizeImage(img image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			srcX := x * img.Bounds().Dx() / width
			srcY := y * img.Bounds().Dy() / height
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

func circularMask(img image.Image) *image.RGBA {
    bounds := img.Bounds()
    w, h := bounds.Dx(), bounds.Dy()
    out := image.NewRGBA(bounds)

    cx, cy := w/2, h/2
    radius := w/2
    if h < w {
        radius = h / 2
    }

    for y := 0; y < h; y++ {
        for x := 0; x < w; x++ {
            dx := x - cx
            dy := y - cy
            if dx*dx+dy*dy <= radius*radius {
                // Copy pixel if inside circle
                out.Set(x, y, img.At(x, y))
            } else {
                // Transparent outside
                out.Set(x, y, color.RGBA{0, 0, 0, 0})
            }
        }
    }
    return out
}


func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
