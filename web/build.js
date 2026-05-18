import * as esbuild from 'esbuild'

await esbuild.build({
  entryPoints: ['src/editor.js'],
  bundle: true,
  minify: true,
  outfile: 'static/editor.js',
  format: 'iife',
  sourcemap: false,
  target: 'es2020',
})
