# Ambiente Claude Code

Referência da configuração usada pelo ambiente de nuvem **marketplace** em
[claude.ai/code](https://claude.ai/code).

## Script de configuração do ambiente

Fica nas configurações do ambiente em claude.ai/code, **não no repositório**.
Roda a cada nova sessão na nuvem:

```bash
#!/bin/bash
LOG=/var/log/setup-ambiente.log

pip install --retries 5 --timeout 60 "playwright==1.56.0" >> "$LOG" 2>&1 \
  || pip install --break-system-packages --retries 5 --timeout 60 "playwright==1.56.0" >> "$LOG" 2>&1 \
  || echo "FALHA: playwright nao instalado" >> "$LOG"

claude plugin marketplace add anthropics/claude-plugins-official >> "$LOG" 2>&1 \
  || echo "FALHA: marketplace oficial nao adicionado" >> "$LOG"

claude plugin install superpowers@claude-plugins-official >> "$LOG" 2>&1 \
  || echo "FALHA: superpowers nao instalada" >> "$LOG"

exit 0
```

Toda a saída dos comandos vai para `/var/log/setup-ambiente.log`. Falhas de
instalação ficam registradas nesse arquivo em linhas que começam com `FALHA`.

### Por que a Superpowers é instalada pelo script

Plugins habilitados apenas via `enabledPlugins` no `.claude/settings.json` do
repositório não são instalados em sessões na nuvem. O script instala o plugin
diretamente, e por isso o repositório não mantém mais um `settings.json`.

### Por que o playwright está fixado em 1.56.0

É a versão do pacote Python compatível com o Chromium pré-instalado no
ambiente (revisão 1194, Chromium 141.0.7390.37, em `/opt/pw-browsers`).
Outras versões tentariam baixar um Chromium diferente.

O pacote Python não publica patch releases: a série 1.56 no PyPI tem apenas
a 1.56.0, que corresponde ao pacote Node `playwright@1.56.1` e usa a mesma
revisão de Chromium. Fixar em `1.56.1` faz o `pip install` falhar com
"No matching distribution found"; no script atual, isso aparece em
`/var/log/setup-ambiente.log` como a linha `FALHA: playwright nao instalado`.

## Skills em `.claude/skills/`

| Skill             | Origem                                                                |
| ----------------- | --------------------------------------------------------------------- |
| `frontend-design` | Copiada de [anthropics/skills](https://github.com/anthropics/skills), commit `34040c9` |
| `webapp-testing`  | Copiada de [anthropics/skills](https://github.com/anthropics/skills), commit `34040c9` |

A skill `skill-creator` **não** fica no repositório: vem sincronizada da conta
do claude.ai e aparece nas sessões como `anthropic-skills:skill-creator`.
