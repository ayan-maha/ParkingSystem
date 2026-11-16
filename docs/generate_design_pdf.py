"""Render the companion Markdown design guide to PDF.

Requires reportlab. Optional PyMuPDF is used separately for visual inspection.
Run from any directory: python docs/generate_design_pdf.py
"""
from pathlib import Path
import html
import re
import argparse

from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import (
    Flowable, KeepInFrame, PageBreak, Paragraph, Preformatted,
    SimpleDocTemplate, Spacer, Table, TableStyle,
)

HERE = Path(__file__).resolve().parent
OUTPUT = HERE / "Smart_Parking_Part_I_Design.pdf"
PAGE_W, PAGE_H = A4
WIDTH = PAGE_W - 88
HEIGHT = PAGE_H - 104
TOTAL_PAGES = 10
DOCUMENT_TITLE = "Smart Parking System - Part I Design and Code Guide"
HEADER = "SMART PARKING / DESIGN & CODE GUIDE"
CLEAN_COVER = False
INK = colors.HexColor("#173147")
TEAL = colors.HexColor("#087e8b")
PALE = colors.HexColor("#eef5f8")

fonts = Path("C:/Windows/Fonts")
if (fonts / "segoeui.ttf").exists():
    pdfmetrics.registerFont(TTFont("Body", str(fonts / "segoeui.ttf")))
    pdfmetrics.registerFont(TTFont("BodyBold", str(fonts / "segoeuib.ttf")))
    pdfmetrics.registerFontFamily("Body", normal="Body", bold="BodyBold", italic="Body", boldItalic="BodyBold")
    FONT, BOLD = "Body", "BodyBold"
else:
    FONT, BOLD = "Helvetica", "Helvetica-Bold"

styles = {
    "body": ParagraphStyle("body", fontName=FONT, fontSize=10, leading=14,
                           textColor=INK, spaceAfter=7, alignment=TA_LEFT),
    "h1": ParagraphStyle("h1", fontName=BOLD, fontSize=21, leading=26,
                         textColor=INK, spaceAfter=15),
    "h2": ParagraphStyle("h2", fontName=BOLD, fontSize=15, leading=20,
                         textColor=TEAL, spaceAfter=10),
    "h3": ParagraphStyle("h3", fontName=BOLD, fontSize=11.3, leading=15,
                         textColor=TEAL, spaceBefore=7, spaceAfter=6),
    "cell": ParagraphStyle("cell", fontName=FONT, fontSize=8.6, leading=11.5,
                           textColor=INK, splitLongWords=True),
    "head": ParagraphStyle("head", fontName=BOLD, fontSize=8.6, leading=11.5,
                           textColor=colors.white),
    "code": ParagraphStyle("code", fontName="Courier", fontSize=8.1, leading=11,
                           textColor=INK, backColor=PALE, borderPadding=9,
                           spaceBefore=4, spaceAfter=12),
}


def inline(text):
    text = html.escape(text)
    text = re.sub(r"`([^`]+)`", r'<font name="Courier" size="9">\1</font>', text)
    return re.sub(r"\*\*([^*]+)\*\*", r"<b>\1</b>", text)


class Architecture(Flowable):
    def __init__(self):
        super().__init__()
        self.width = WIDTH
        self.height = 117

    def draw(self):
        c = self.canv
        boxes = [(0, "CLIENT", "curl / Swagger"),
                 (181, "REST HANDLER", "HTTP adapter + gRPC client"),
                 (362, "gRPC SERVICE", "Part I dummy implementation")]
        for x, title, sub in boxes:
            c.setFillColor(PALE)
            c.setStrokeColor(TEAL)
            c.roundRect(x, 43, 145, 53, 6, fill=1, stroke=1)
            c.setFillColor(INK)
            c.setFont(BOLD, 10)
            c.drawCentredString(x + 72.5, 74, title)
            c.setFont(FONT, 7.7)
            c.drawCentredString(x + 72.5, 57, sub)
        for start, end in [(145, 181), (326, 362)]:
            c.setStrokeColor(TEAL)
            c.line(start + 3, 70, end - 4, 70)
            c.line(end - 9, 73, end - 4, 70)
            c.line(end - 9, 67, end - 4, 70)
        c.setFillColor(TEAL)
        c.setFont(BOLD, 8.5)
        c.drawCentredString(162, 27, "HTTP / JSON")
        c.drawCentredString(344, 27, "gRPC / protobuf")
        c.setFont(FONT, 8)
        c.setFillColor(INK)
        c.drawCentredString(WIDTH / 2, 8, "Replies return through the same layers in reverse order.")


class ReportCover(Flowable):
    def __init__(self):
        super().__init__()
        self.width = WIDTH
        self.height = 660

    def draw(self):
        c = self.canv
        for y, name, identifier in [(626, "Hala Atamna", "213620651"),
                                    (593, "Ayan Mahameed", "214556532")]:
            c.setFillColor(INK)
            c.setFont(BOLD, 17)
            c.drawString(0, y, name)
            c.setFillColor(TEAL)
            c.setFont(FONT, 13)
            c.drawRightString(WIDTH, y, "ID " + identifier)


def make_table(lines):
    rows = [[x.strip() for x in line.strip().strip("|").split("|")] for line in lines]
    rows = [row for row in rows if not all(re.fullmatch(r":?-+:?", cell) for cell in row)]
    n = len(rows[0])
    if n == 4:
        proportions = [0.43, 0.26, 0.12, 0.19]
    elif n == 3:
        proportions = [0.26, 0.30, 0.44]
        if rows[0][0] == "REST operation":
            proportions = [0.48, 0.32, 0.20]
    else:
        proportions = [0.27, 0.73]
    data = [[Paragraph(inline(cell), styles["head" if i == 0 else "cell"])
             for cell in row] for i, row in enumerate(rows)]
    for i, row in enumerate(rows[1:], start=1):
        swatch = re.search(r"#[0-9A-Fa-f]{6}", row[0])
        if swatch:
            label_color = "#173147" if swatch[0].upper() == "#EEF5F8" else swatch[0]
            data[i][0] = Paragraph(f'<font color="{label_color}"><b>{inline(row[0])}</b></font>', styles["cell"])
    table = Table(data, colWidths=[WIDTH * p for p in proportions], hAlign="LEFT")
    table.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), INK),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, PALE]),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("LEFTPADDING", (0, 0), (-1, -1), 7),
        ("RIGHTPADDING", (0, 0), (-1, -1), 7),
        ("TOPPADDING", (0, 0), (-1, -1), 6),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
        ("LINEBELOW", (0, -1), (-1, -1), 0.5, colors.HexColor("#ccdbe3")),
    ]))
    return [table, Spacer(1, 9)]


def parse_page(text):
    lines = text.strip().splitlines()
    result = []
    i = 0
    while i < len(lines):
        line = lines[i]
        if not line.strip():
            i += 1
            continue
        if line.startswith("```"):
            code = []
            i += 1
            while i < len(lines) and not lines[i].startswith("```"):
                code.append(lines[i])
                i += 1
            result.append(Preformatted("\n".join(code), styles["code"]))
            i += 1
            continue
        if line.startswith("|"):
            table_lines = []
            while i < len(lines) and lines[i].startswith("|"):
                table_lines.append(lines[i])
                i += 1
            result.extend(make_table(table_lines))
            continue
        if line == "{{architecture}}":
            result.append(Architecture())
            i += 1
            continue
        if line == "{{cover}}":
            result.append(ReportCover())
            i += 1
            continue
        match = re.match(r"^(#{1,3}) (.*)$", line)
        if match:
            result.append(Paragraph(inline(match[2]), styles["h" + str(len(match[1]))]))
            i += 1
            continue
        if re.match(r"^\d+\. ", line):
            result.append(Paragraph(inline(line), styles["body"]))
            i += 1
            continue
        paragraph = [line]
        i += 1
        while i < len(lines) and lines[i].strip() and not re.match(r"^(#|\||```|\d+\. )", lines[i]):
            paragraph.append(lines[i])
            i += 1
        result.append(Paragraph(inline(" ".join(paragraph)), styles["body"]))
    return result


def decorate(canvas, doc):
    canvas.saveState()
    canvas.setTitle(DOCUMENT_TITLE)
    canvas.setAuthor("Smart Parking System project")
    if CLEAN_COVER and doc.page == 1:
        canvas.restoreState()
        return
    canvas.setFont(BOLD, 8)
    canvas.setFillColor(TEAL)
    canvas.drawString(44, PAGE_H - 29, HEADER)
    canvas.setStrokeColor(colors.HexColor("#ccdbe3"))
    canvas.line(44, 37, PAGE_W - 44, 37)
    canvas.setFont(FONT, 8)
    canvas.setFillColor(INK)
    canvas.drawString(44, 23, "Distributed Systems - Part I | 16 November 2026")
    canvas.drawRightString(PAGE_W - 44, 23, f"{doc.page} / {TOTAL_PAGES}")
    canvas.restoreState()


def main():
    global TOTAL_PAGES, DOCUMENT_TITLE, HEADER, INK, TEAL, PALE, CLEAN_COVER
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=HERE / "design.md")
    parser.add_argument("--output", type=Path, default=OUTPUT)
    parser.add_argument("--report", action="store_true")
    parser.add_argument("--blue-brown", action="store_true")
    args = parser.parse_args()
    if args.blue_brown:
        INK = colors.HexColor("#183A61")
        TEAL = colors.HexColor("#805637")
        PALE = colors.HexColor("#F4F0EB")
        for name in ("body", "h1", "cell", "code"):
            styles[name].textColor = INK
        for name in ("h2", "h3"):
            styles[name].textColor = TEAL
        styles["code"].backColor = PALE
    if args.report:
        DOCUMENT_TITLE = "Smart Parking System - Part I Project Report"
        HEADER = "SMART PARKING / PART I PROJECT REPORT"
    text = args.source.read_text(encoding="utf-8")
    CLEAN_COVER = text.strip().startswith("{{cover}}")
    pages = text.split("<!-- page -->")
    TOTAL_PAGES = len(pages)
    story, frames = [], []
    for index, page in enumerate(pages):
        if index:
            story.append(PageBreak())
        frame = KeepInFrame(WIDTH, HEIGHT - 12, parse_page(page), mode="shrink", hAlign="LEFT", vAlign="TOP")
        frames.append(frame)
        story.append(frame)
    doc = SimpleDocTemplate(str(args.output), pagesize=A4, leftMargin=44, rightMargin=44,
                            topMargin=52, bottomMargin=52)
    doc.build(story, onFirstPage=decorate, onLaterPages=decorate)
    print(args.output)
    print("Page scale divisors:", [round(getattr(frame, "_scale", 1), 3) for frame in frames])


if __name__ == "__main__":
    main()
