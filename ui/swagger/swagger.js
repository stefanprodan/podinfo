// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: Apache-2.0

const dark = window.matchMedia("(prefers-color-scheme: dark)");
const setTheme = () => {
  document.documentElement.classList.toggle("dark-mode", dark.matches);
};
dark.addEventListener("change", setTheme);
setTheme();

window.ui = SwaggerUIBundle({
  url: "doc.json",
  dom_id: "#swagger-ui",
  deepLinking: true,
  validatorUrl: null,
});
