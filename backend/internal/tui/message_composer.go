package tui

import (
	"image"
	"strings"
	rw "github.com/mattn/go-runewidth"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
)

// messageComposer wraps long drafts and scrolls vertically to keep the cursor visible.
type messageComposer struct { *widgets.TextArea; top int }
func newMessageComposer()*messageComposer {a:=widgets.NewTextArea();a.Border=true;a.Title="Message";a.CursorStyle=focusedStyle();return &messageComposer{TextArea:a}}
func (c *messageComposer) visualLines(width int) int {rows,_:=layoutMessage(c.Text,c.Cursor,max(1,width));return len(rows)}
func (c *messageComposer) Draw(buf *ui.Buffer) {
	c.Block.Draw(buf)
	width,height:=c.Inner.Dx(),c.Inner.Dy();if width<=0 || height<=0{return}
	rows,cursor:=layoutMessage(c.Text,c.Cursor,width)
	if cursor.Y<c.top {c.top=cursor.Y};if cursor.Y>=c.top+height {c.top=cursor.Y-height+1};c.top=min(max(0,c.top),max(0,len(rows)-height))
	for y:=0;y<height && c.top+y<len(rows);y++ {x:=c.Inner.Min.X;for _,r:=range rows[c.top+y] {w:=rw.RuneWidth(r);if x+w>c.Inner.Max.X {break};buf.SetCell(ui.NewCell(r,c.TextStyle),image.Pt(x,c.Inner.Min.Y+y));x+=w}}
	if c.ShowCursor && cursor.Y>=c.top && cursor.Y<c.top+height && cursor.X<width {p:=image.Pt(c.Inner.Min.X+cursor.X,c.Inner.Min.Y+cursor.Y-c.top);cell:=buf.GetCell(p);if cell.Rune==0 {cell.Rune=' '};cell.Style=c.CursorStyle;buf.SetCell(cell,p)}
}

func layoutMessage(text string,cursor image.Point,width int)([]string,image.Point){
	var rows []string;visualY:=0;position:=image.Point{}
	logical:=strings.Split(text,"\n")
	for y,line:=range logical {
		chars:=[]rune(line);var part []rune;used:=0;partY:=visualY
		for i,r:=range chars {
			w:=rw.RuneWidth(r);if len(part)>0 && used+w>width {rows=append(rows,string(part));visualY++;partY=visualY;part=nil;used=0}
			if cursor.Y==y && cursor.X==i {position=image.Pt(used,visualY)}
			part=append(part,r);used+=w
		}
		rows=append(rows,string(part))
		if cursor.Y==y && cursor.X>=len(chars) {if used>=width && y==len(logical)-1 {position=image.Pt(0,visualY+1)} else {position=image.Pt(used,visualY)}}
		visualY++
		_ = partY
	}
	if position.Y>=len(rows) {rows=append(rows,"")}
	return rows,position
}
