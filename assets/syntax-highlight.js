(() => {
  "use strict";

  const tokenPattern = /(\s+)|(#[^\n]*)|("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')|(https?:\/\/[^\s"'|&]+)|(\$\{?[A-Za-z_][A-Za-z0-9_]*\}?|%[A-Za-z_][A-Za-z0-9_]*%)|(--?[A-Za-z0-9][A-Za-z0-9-]*|-[A-Z])|(&&|\|\||[|;&<>]|\\(?=\s*$))|([^\s"'|&;<>]+)/gy;

  const sourceText = (code) => Array.from(code.childNodes, (node) =>
    node.nodeName === "BR" ? "\n" : node.textContent
  ).join("");

  const addToken = (fragment, value, type) => {
    if (!type) {
      fragment.append(document.createTextNode(value));
      return;
    }
    const token = document.createElement("span");
    token.className = `sh-${type}`;
    token.textContent = value;
    fragment.append(token);
  };

  document.querySelectorAll("pre > code").forEach((code) => {
    if (code.querySelector("span")) return;

    const source = sourceText(code);
    const fragment = document.createDocumentFragment();
    let expectCommand = true;
    let match;
    tokenPattern.lastIndex = 0;

    while ((match = tokenPattern.exec(source))) {
      const value = match[0];
      let type = "";
      if (match[1]) {
        if (value.includes("\n")) expectCommand = true;
      } else if (match[2]) {
        type = "comment";
      } else if (match[3]) {
        type = "string";
        expectCommand = false;
      } else if (match[4]) {
        type = "url";
        expectCommand = false;
      } else if (match[5]) {
        type = "variable";
        expectCommand = false;
      } else if (match[6]) {
        type = "option";
        expectCommand = false;
      } else if (match[7]) {
        type = "operator";
        if (value !== "\\") expectCommand = true;
      } else if (expectCommand) {
        type = "command";
        expectCommand = false;
      }
      addToken(fragment, value, type);
    }

    code.replaceChildren(fragment);
    code.parentElement.classList.add("syntax-shell");
  });
})();
