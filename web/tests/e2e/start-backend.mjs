import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { execFileSync, spawn } from 'node:child_process';

const root = mkdtempSync(path.join(tmpdir(), 'dnl-e2e-'));
const vault = path.join(root, 'vault');
mkdirSync(vault);
writeFileSync(path.join(vault, '.daneel.yaml'), 'vault:\n  title: Demonstration Vault\n  owner: Ada Lovelace\n  indexed_dirs: [notes, daily, meta, inbox, sources]\nworkspace:\n  work_dir: ./empty-workspace\n');
for (const category of ['architecture', 'engineering', 'research']) {
  const dir = path.join(vault, 'notes', category);
  mkdirSync(dir, { recursive: true });
  for (let i = 0; i < 16; i++) {
    const next = (i + 1) % 16;
    const readingDetails = i === 0 ? '\n\n' + Array.from({ length: 24 }, (_, paragraph) =>
      `Reading example ${paragraph + 1}: durable cache design keeps local knowledge available and documents the reasoning behind each engineering decision.`
    ).join('\n\n') : '';
    writeFileSync(path.join(dir, `note-${i}.md`), `---\ntitle: ${category} example ${i}\ntype: technology\n---\n# ${category} example ${i}\n\nCache and system design notes.\n\n[Next example](note-${next}.md)\n[Related research](../research/note-${i}.md)\n${readingDetails}`);
  }
}
for (const repository of ['orion', 'atlas']) {
  const dir = path.join(vault, 'sources', 'github', 'acme', repository);
  mkdirSync(dir, { recursive: true });
  for (let i = 0; i < 4; i++) {
    writeFileSync(path.join(dir, `note-${i}.md`), `---\ntitle: ${repository} guide ${i}\ntype: source\n---\n# ${repository} guide ${i}\n\nA neutral repository guide about gateway architecture.\n\n[Next guide](note-${(i + 1) % 4}.md)\n`);
  }
}
const dailyDir = path.join(vault, 'daily');
mkdirSync(dailyDir);
for (const date of ['2026-10-07', '2026-10-16']) {
  writeFileSync(path.join(dailyDir, `${date}.md`), `---\ntitle: Daily log for ${date}\n---\n# Daily log for ${date}\n\nA neutral daily reflection.\n`);
}
const binary = path.join(root, process.platform === 'win32' ? 'daneel.exe' : 'daneel');
const env = { ...process.env, DANEEL_CONFIG: '', DANEEL_VAULT_PATH: '', CGO_ENABLED: '0' };
for (const kind of ['CONFIG', 'CACHE', 'DATA', 'STATE']) env[`XDG_${kind}_HOME`] = path.join(root, kind.toLowerCase());
execFileSync('go', ['build', '-o', binary, '..'], { env, stdio: 'inherit' });
const child = spawn(binary, ['serve', '--port', '8088', '--vault-path', vault], { env, stdio: 'inherit' });
let stopping = false;
const stop = () => { stopping = true; child.kill(); };
process.on('SIGINT', stop);
process.on('SIGTERM', stop);
child.on('exit', (code) => {
  rmSync(root, { recursive: true, force: true });
  process.exit(stopping ? 0 : (code ?? 1));
});
