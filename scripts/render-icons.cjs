// npm install --prefix /tmp/xiterm-icon-tools @resvg/resvg-js@2.6.2
// NODE_PATH=/tmp/xiterm-icon-tools/node_modules node scripts/render-icons.cjs
const fs = require('node:fs');
const path = require('node:path');
const { Resvg } = require('@resvg/resvg-js');
const root = path.resolve(__dirname, '..');
const svg = fs.readFileSync(path.join(root, 'assets/icons/xiterm.svg'));
for (const size of [512, 1024]) {
  const renderer = new Resvg(svg, { fitTo: { mode: 'width', value: size } });
  const name = size === 512 ? 'xiterm.png' : 'xiterm-1024.png';
  fs.writeFileSync(path.join(root, 'assets/icons', name), renderer.render().asPng());
}
