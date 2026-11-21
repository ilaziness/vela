package main

import (
	"image"
	"math"

	"github.com/go-gui-org/go-glyph"
)

// softTexture is one glyph atlas page held in memory as straight RGBA.
type softTexture struct {
	pix  []byte
	w, h int
}

// softBackend implements glyph.DrawBackend over an image.NRGBA framebuffer,
// so go-glyph text can be rendered without a GPU or native window.
type softBackend struct {
	img      *image.NRGBA
	textures map[glyph.TextureID]*softTexture
	nextID   glyph.TextureID
	scale    float32
}

func newSoftBackend(img *image.NRGBA, scale float32) *softBackend {
	return &softBackend{
		img:      img,
		textures: make(map[glyph.TextureID]*softTexture),
		scale:    scale,
	}
}

func (b *softBackend) NewTexture(width, height int) glyph.TextureID {
	if width <= 0 || height <= 0 {
		return 0
	}
	b.nextID++
	b.textures[b.nextID] = &softTexture{
		pix: make([]byte, width*height*4),
		w:   width,
		h:   height,
	}
	return b.nextID
}

func (b *softBackend) UpdateTexture(id glyph.TextureID, data []byte) {
	tex, ok := b.textures[id]
	if !ok || len(data) == 0 {
		return
	}
	copy(tex.pix, data)
}

// UpdateTextureRect implements glyph.RectTextureUpdater so glyph uploads new
// atlas pages mid-frame, before the quads that sample them are drawn.
func (b *softBackend) UpdateTextureRect(id glyph.TextureID, data []byte,
	srcStride, x, y, w, h int) {

	tex, ok := b.textures[id]
	if !ok || w <= 0 || h <= 0 || x < 0 || y < 0 ||
		w > tex.w-x || h > tex.h-y || srcStride < w*4 {
		return
	}
	rowEnd := (x + w) * 4
	if rowEnd > len(data) || y+h-1 > (len(data)-rowEnd)/srcStride {
		return
	}
	rowBytes := w * 4
	dstStride := tex.w * 4
	for row := y; row < y+h; row++ {
		so := row*srcStride + x*4
		do := row*dstStride + x*4
		copy(tex.pix[do:do+rowBytes], data[so:so+rowBytes])
	}
}

func (b *softBackend) DeleteTexture(id glyph.TextureID) {
	delete(b.textures, id)
}

func (b *softBackend) DPIScale() float32 { return b.scale }

func (b *softBackend) DrawTexturedQuad(id glyph.TextureID,
	src, dst glyph.Rect, c glyph.Color) {

	tex, ok := b.textures[id]
	if !ok || c.A == 0 {
		return
	}
	s := b.scale
	x0, y0 := dst.X*s, dst.Y*s
	x1, y1 := (dst.X+dst.Width)*s, (dst.Y+dst.Height)*s
	if x1 <= x0 || y1 <= y0 {
		return
	}
	invW := 1 / (x1 - x0)
	invH := 1 / (y1 - y0)
	for py := int(math.Ceil(float64(y0))); py < int(math.Ceil(float64(y1))); py++ {
		v := (float32(py) + 0.5 - y0) * invH
		sy := int(src.Y + v*src.Height)
		for px := int(math.Ceil(float64(x0))); px < int(math.Ceil(float64(x1))); px++ {
			u := (float32(px) + 0.5 - x0) * invW
			sx := int(src.X + u*src.Width)
			b.blendTexel(tex, sx, sy, px, py, c)
		}
	}
}

func (b *softBackend) DrawTexturedQuadTransformed(id glyph.TextureID,
	src, dst glyph.Rect, c glyph.Color, t glyph.AffineTransform) {

	tex, ok := b.textures[id]
	if !ok || c.A == 0 || dst.Width <= 0 || dst.Height <= 0 {
		return
	}
	det := t.XX*t.YY - t.XY*t.YX
	if det == 0 {
		return
	}
	s := b.scale
	corners := [4][2]float32{
		{dst.X, dst.Y},
		{dst.X + dst.Width, dst.Y},
		{dst.X + dst.Width, dst.Y + dst.Height},
		{dst.X, dst.Y + dst.Height},
	}
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := -float32(math.MaxFloat32), -float32(math.MaxFloat32)
	for i := range corners {
		cx, cy := t.Apply(corners[i][0], corners[i][1])
		cx *= s
		cy *= s
		minX, maxX = min(minX, cx), max(maxX, cx)
		minY, maxY = min(minY, cy), max(maxY, cy)
	}
	if math.IsNaN(float64(minX)) || math.IsNaN(float64(minY)) {
		return
	}
	x0, y0 := int(math.Floor(float64(minX))), int(math.Floor(float64(minY)))
	x1, y1 := int(math.Ceil(float64(maxX))), int(math.Ceil(float64(maxY)))

	invDet := 1 / det
	ixx, ixy := t.YY*invDet, -t.XY*invDet
	iyx, iyy := -t.YX*invDet, t.XX*invDet
	invS := 1 / s

	for py := y0; py < y1; py++ {
		dy := (float32(py) + 0.5) * invS
		for px := x0; px < x1; px++ {
			dx := (float32(px) + 0.5) * invS
			ox, oy := dx-t.X0, dy-t.Y0
			lx := ixx*ox + ixy*oy
			ly := iyx*ox + iyy*oy
			u := (lx - dst.X) / dst.Width
			v := (ly - dst.Y) / dst.Height
			if u < 0 || u >= 1 || v < 0 || v >= 1 {
				continue
			}
			sx := int(src.X + u*src.Width)
			sy := int(src.Y + v*src.Height)
			b.blendTexel(tex, sx, sy, px, py, c)
		}
	}
}

func (b *softBackend) DrawFilledRect(dst glyph.Rect, c glyph.Color) {
	if c.A == 0 {
		return
	}
	s := b.scale
	x0 := int(math.Floor(float64(dst.X * s)))
	y0 := int(math.Floor(float64(dst.Y * s)))
	x1 := int(math.Ceil(float64((dst.X + dst.Width) * s)))
	y1 := int(math.Ceil(float64((dst.Y + dst.Height) * s)))
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			b.blendPixel(px, py, c)
		}
	}
}

func (b *softBackend) DrawFilledRectTransformed(dst glyph.Rect,
	c glyph.Color, t glyph.AffineTransform) {

	if c.A == 0 {
		return
	}
	corners := [4][2]float32{
		{dst.X, dst.Y},
		{dst.X + dst.Width, dst.Y},
		{dst.X + dst.Width, dst.Y + dst.Height},
		{dst.X, dst.Y + dst.Height},
	}
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := -float32(math.MaxFloat32), -float32(math.MaxFloat32)
	var pts [4][2]float32
	for i := range corners {
		cx, cy := t.Apply(corners[i][0], corners[i][1])
		cx *= b.scale
		cy *= b.scale
		pts[i] = [2]float32{cx, cy}
		minX, maxX = min(minX, cx), max(maxX, cx)
		minY, maxY = min(minY, cy), max(maxY, cy)
	}
	x0 := int(math.Floor(float64(minX)))
	y0 := int(math.Floor(float64(minY)))
	x1 := int(math.Ceil(float64(maxX)))
	y1 := int(math.Ceil(float64(maxY)))
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			// Point-in-parallelogram test in device space.
			fx := float32(px) + 0.5
			fy := float32(py) + 0.5
			e1x, e1y := pts[1][0]-pts[0][0], pts[1][1]-pts[0][1]
			e2x, e2y := pts[3][0]-pts[0][0], pts[3][1]-pts[0][1]
			denom := e1x*e2y - e1y*e2x
			if denom == 0 {
				return
			}
			dx, dy := fx-pts[0][0], fy-pts[0][1]
			u := (dx*e2y - dy*e2x) / denom
			v := (e1x*dy - e1y*dx) / denom
			if u >= 0 && u <= 1 && v >= 0 && v <= 1 {
				b.blendPixel(px, py, c)
			}
		}
	}
}

// blendTexel composites one tinted atlas texel onto a device pixel using
// source-over alpha blending, matching the GPU backends.
func (b *softBackend) blendTexel(tex *softTexture, sx, sy, px, py int,
	c glyph.Color) {

	if sx < 0 || sy < 0 || sx >= tex.w || sy >= tex.h {
		return
	}
	i := (sy*tex.w + sx) * 4
	ta := uint32(tex.pix[i+3])
	if ta == 0 {
		return
	}
	sr := uint32(tex.pix[i]) * uint32(c.R) / 255
	sg := uint32(tex.pix[i+1]) * uint32(c.G) / 255
	sb := uint32(tex.pix[i+2]) * uint32(c.B) / 255
	sa := ta * uint32(c.A) / 255
	b.blendPixelRaw(px, py, sr, sg, sb, sa)
}

func (b *softBackend) blendPixel(px, py int, c glyph.Color) {
	b.blendPixelRaw(px, py,
		uint32(c.R), uint32(c.G), uint32(c.B), uint32(c.A))
}

func (b *softBackend) blendPixelRaw(px, py int,
	sr, sg, sb, sa uint32) {

	img := b.img
	bounds := img.Bounds()
	if px < bounds.Min.X || py < bounds.Min.Y ||
		px >= bounds.Max.X || py >= bounds.Max.Y || sa == 0 {
		return
	}
	ia := 255 - sa
	i := (py*bounds.Dx() + (px - bounds.Min.X)) * 4
	img.Pix[i+0] = uint8((sr*sa + uint32(img.Pix[i+0])*ia + 127) / 255)
	img.Pix[i+1] = uint8((sg*sa + uint32(img.Pix[i+1])*ia + 127) / 255)
	img.Pix[i+2] = uint8((sb*sa + uint32(img.Pix[i+2])*ia + 127) / 255)
	img.Pix[i+3] = uint8(sa + uint32(img.Pix[i+3])*ia/255)
}
