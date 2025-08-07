import json
import os
from pathlib import Path



LAUNCH_PATH = Path(__file__).parent / "launch.json"
CMD_DIR = Path(__file__).parent.parent / "cmd"

print("LAUNCH_PATH: ", LAUNCH_PATH)
print("CMD_DIR: ", CMD_DIR)


def get_go_files(max_depth=5):
    go_files = []
    base_path = CMD_DIR.resolve()
    for f in CMD_DIR.rglob("*.go"):
        if f.is_file():
            relative = f.relative_to(base_path)
            if len(relative.parts) <= max_depth:
                go_files.append(str(relative))
                
    return sorted(go_files)

def update_launch_json(files):
    if not LAUNCH_PATH.exists():
        print(f"{LAUNCH_PATH} não encontrado.")
        return

    with open(LAUNCH_PATH, "r", encoding="utf-8") as f:
        data = json.load(f)

    inputs = data.get("inputs", [])
    for input_item in inputs:
        if input_item.get("id") == "arquivoSelecionado":
            input_item["options"] = files

    with open(LAUNCH_PATH, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=4)

    print(f"`inputs.options` atualizado com {len(files)} arquivos.")

if __name__ == "__main__":
    arquivos = get_go_files()
    if not arquivos:
        print("Nenhum arquivo .go encontrado em cmd/")
    else:
        update_launch_json(arquivos)
