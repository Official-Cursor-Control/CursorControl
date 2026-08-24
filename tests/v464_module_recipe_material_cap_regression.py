from pathlib import Path
import re

SRC = Path(__file__).resolve().parents[1]
text = (SRC / "afk_modules.go").read_text()

start = text.index("var afkModuleRecipes = [afkModuleItemCount][afkCraftComponentCount]int{")
end = text.index("\n}\n\nvar afkModulePanelOpen", start)
block = text[start:end]
rows = []
for match in re.finditer(r"\{([^{}]+)\}", block.split("\n", 1)[1]):
    nums = [int(x.strip()) for x in match.group(1).split(",") if x.strip()]
    if len(nums) == 6:
        rows.append(nums)

assert len(rows) == 72, f"expected 72 module recipes, found {len(rows)}"
for idx, row in enumerate(rows):
    used = sum(1 for amount in row if amount > 0)
    assert used <= 3, f"module recipe {idx} uses {used} material types: {row}"

print("PASS: all 72 ship-module recipes use at most 3 material types")
