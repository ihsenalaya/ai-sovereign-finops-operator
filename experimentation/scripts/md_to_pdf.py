#!/usr/bin/env python3
"""Render paper/paper.md to a PDF (pure-Python, fpdf2 + DejaVu), embedding the
figures. Not a typesetting engine — produces a clean, readable draft PDF.

Usage: python3 scripts/md_to_pdf.py [--md paper/paper.md] [--out paper/paper.pdf] [--figures figures]
"""
import argparse
import os
import re
from fpdf import FPDF

FONT_DIR = "/usr/share/fonts/truetype/dejavu"


class PDF(FPDF):
    def header(self):
        pass

    def footer(self):
        self.set_y(-12)
        self.set_font("DejaVu", "", 8)
        self.set_text_color(120)
        self.cell(0, 8, f"{self.page_no()}", align="C")
        self.set_text_color(0)


def setup(pdf):
    pdf.add_font("DejaVu", "", f"{FONT_DIR}/DejaVuSans.ttf")
    pdf.add_font("DejaVu", "B", f"{FONT_DIR}/DejaVuSans-Bold.ttf")
    pdf.add_font("DejaVu", "I", f"{FONT_DIR}/DejaVuSans.ttf")
    pdf.add_font("DejaVu", "BI", f"{FONT_DIR}/DejaVuSans-Bold.ttf")
    pdf.add_font("Mono", "", f"{FONT_DIR}/DejaVuSansMono.ttf")


def render_table(pdf, rows):
    if not rows:
        return
    rows = [[c.strip() for c in r] for r in rows]
    ncol = max(len(r) for r in rows)
    rows = [r + [""] * (ncol - len(r)) for r in rows]
    avail = pdf.w - pdf.l_margin - pdf.r_margin
    w = avail / ncol
    pdf.set_font("DejaVu", "", 7.5)
    for i, r in enumerate(rows):
        style = "B" if i == 0 else ""
        pdf.set_font("DejaVu", style, 7.5)
        # compute row height by wrapping
        line_h = 4.2
        cells = [pdf.multi_cell(w, line_h, c, border=0, align="L", dry_run=True, output="LINES") for c in r]
        nlines = max(len(c) for c in cells) or 1
        h = nlines * line_h
        if pdf.get_y() + h > pdf.h - pdf.b_margin:
            pdf.add_page()
        x0, y0 = pdf.get_x(), pdf.get_y()
        for j, c in enumerate(r):
            x = x0 + j * w
            pdf.set_xy(x, y0)
            pdf.multi_cell(w, line_h, c, border=1, align="L", max_line_height=line_h)
        pdf.set_xy(x0, y0 + h)
    pdf.ln(2)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--md", default="paper/paper.md")
    ap.add_argument("--out", default="paper/paper.pdf")
    ap.add_argument("--figures", default="figures")
    args = ap.parse_args()

    with open(args.md, encoding="utf-8") as fh:
        lines = fh.read().splitlines()

    pdf = PDF(format="A4")
    pdf.set_auto_page_break(True, margin=15)
    pdf.set_margins(18, 16, 18)
    setup(pdf)
    pdf.add_page()
    width = pdf.w - pdf.l_margin - pdf.r_margin

    in_code = False
    code_buf = []
    table_buf = []

    def flush_table():
        nonlocal table_buf
        if table_buf:
            render_table(pdf, table_buf)
            table_buf = []

    for raw in lines:
        line = raw.rstrip("\n")

        if line.strip().startswith("```"):
            if in_code:
                pdf.set_font("Mono", "", 7.5)
                pdf.set_fill_color(244, 244, 244)
                for cl in code_buf:
                    pdf.multi_cell(width, 4, cl, fill=True)
                pdf.ln(1)
                code_buf = []
                in_code = False
            else:
                flush_table()
                in_code = True
            continue
        if in_code:
            code_buf.append(line)
            continue

        if line.lstrip().startswith("|") and "|" in line[1:]:
            cells = [c for c in line.strip().strip("|").split("|")]
            if set(line.replace("|", "").replace("-", "").replace(":", "").strip()) == set():
                continue  # separator row
            table_buf.append(cells)
            continue
        else:
            flush_table()

        s = line.strip()
        if not s:
            pdf.ln(2.5)
            continue
        if s.startswith("# "):
            pdf.set_font("DejaVu", "B", 16)
            pdf.multi_cell(width, 8, s[2:], markdown=True)
            pdf.ln(1)
        elif s.startswith("## "):
            pdf.ln(1)
            pdf.set_font("DejaVu", "B", 12.5)
            pdf.multi_cell(width, 6.5, s[3:], markdown=True)
            pdf.ln(0.5)
        elif s.startswith("### "):
            pdf.set_font("DejaVu", "B", 10.5)
            pdf.multi_cell(width, 5.5, s[4:], markdown=True)
        elif s.startswith("---"):
            pdf.ln(1)
        elif s.startswith("$$") or s.endswith("$$"):
            pdf.set_font("Mono", "", 8)
            pdf.multi_cell(width, 4.5, s.replace("$$", "").strip())
        elif s.startswith("- ") or s.startswith("* "):
            pdf.set_font("DejaVu", "", 9.5)
            pdf.multi_cell(width, 5, "  •  " + s[2:], markdown=True)
        elif re.match(r"^\d+\.\s", s):
            pdf.set_font("DejaVu", "", 9.5)
            pdf.multi_cell(width, 5, "  " + s, markdown=True)
        else:
            pdf.set_font("DejaVu", "", 9.5)
            pdf.multi_cell(width, 5, s, markdown=True)

    flush_table()

    # Figures appendix
    figs = [
        ("fig2_cost_by_strategy.png", "Figure 2 — Total cost by routing strategy"),
        ("fig3_quality_vs_cost.png", "Figure 3 — Quality vs cost"),
        ("fig4_latency.png", "Figure 4 — Latency p95 and mean ±95% CI"),
        ("fig5_sovereignty.png", "Figure 5 — Sovereignty: cost & violations"),
        ("fig6_budget.png", "Figure 6 — Budget policy: availability vs overrun"),
        ("fig7_breakeven.png", "Figure 7 — Managed vs self-hosted break-even (modeled)"),
        ("fig8_ablation.png", "Figure 8 — Ablation of scoring terms"),
    ]
    pdf.add_page()
    pdf.set_font("DejaVu", "B", 14)
    pdf.multi_cell(width, 8, "Appendix: Figures")
    pdf.ln(2)
    for fn, cap in figs:
        path = os.path.join(args.figures, fn)
        if not os.path.exists(path):
            continue
        if pdf.get_y() > pdf.h - 90:
            pdf.add_page()
        pdf.set_font("DejaVu", "B", 9.5)
        pdf.multi_cell(width, 5, cap)
        pdf.image(path, w=width)
        pdf.ln(4)

    pdf.output(args.out)
    print("wrote", args.out, f"({os.path.getsize(args.out)//1024} KB, {pdf.page_no()} pages)")


if __name__ == "__main__":
    main()
