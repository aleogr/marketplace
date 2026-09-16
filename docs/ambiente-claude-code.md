# Ambiente Claude Code

Referência da configuração usada pelo ambiente de nuvem **marketplace** em
[claude.ai/code](https://claude.ai/code).

## Script de configuração do ambiente

Fica nas configurações do ambiente em claude.ai/code, **não no repositório**.
Roda a cada nova sessão na nuvem:

```bash
#!/bin/bash
pip install "playwright==1.56.1" || pip install --break-system-packages "playwright==1.56.1" || true
claude plugin marketplace add anthropics/claude-plugins-official || true
claude plugin install superpowers@claude-plugins-official || true
```

### Por que a Superpowers é instalada pelo script

Plugins habilitados apenas via `enabledPlugins` no `.claude/settings.json` do
repositório não são instalados em sessões na nuvem. O script instala o plugin
diretamente, e por isso o repositório não mantém mais um `settings.json`.

### Por que o playwright está fixado em 1.56.1

É a versão compatível com o Chromium pré-instalado no ambiente (build 1194,
em `/opt/pw-browsers`). Outras versões tentariam baixar um Chromium diferente.

## Skills em `.claude/skills/`

| Skill             | Origem                                                                |
| ----------------- | --------------------------------------------------------------------- |
| `frontend-design` | Copiada de [anthropics/skills](https://github.com/anthropics/skills), commit `34040c9` |
| `webapp-testing`  | Copiada de [anthropics/skills](https://github.com/anthropics/skills), commit `34040c9` |

A skill `skill-creator` **não** fica no repositório: vem sincronizada da conta
do claude.ai e aparece nas sessões como `anthropic-skills:skill-creator`.
