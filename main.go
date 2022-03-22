package main

import "C"
import (
    "github.com/charmbracelet/glamour"
)

//export Glamourify
func Glamourify(body string, width int) *C.char {
   r, _ := glamour.NewTermRenderer(
       glamour.WithStandardStyle("dark"),
       glamour.WithWordWrap(width),
   )
   out, _ := r.Render(body)
   return C.CString(out)
}

func main() { }
