# Lybel Skills

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/lybel-app/skills?color=11C47E&label=release)](https://github.com/lybel-app/skills/releases/latest)
[![Claude Skills](https://img.shields.io/badge/Claude-Skills-11C47E)](https://docs.claude.com/en/docs/claude-code/skills)

> Skills open-source do Claude mantidas pelo time da **Lybel**. Funcionam pra qualquer empresa — basta apontar pro seu Confluence/Jira/etc. PRs bem-vindos.

## Skills disponíveis

| Skill | Resumo | Docs |
|---|---|---|
| **`confluence-docs`** | Busca, cria e atualiza páginas do Confluence em linguagem natural. Usa CLI Go local que devolve digests/sections em vez do ADF inteiro — 10–50× mais barato em tokens que o MCP puro (que fica como fallback). | [SKILL.md](./confluence-docs/SKILL.md) |

Próximas candidatas: `jira-tickets`, `figma-files`, `analytics`.

---

## Como funciona

Skills aqui são **timeless**: o repo só guarda estrutura, workflows e templates — nenhum dado da Lybel (advisors, investidores, page IDs específicos). Em runtime, o Claude consulta a Home do Confluence (pageId `164232`), que é a fonte da verdade da taxonomia e do índice. Por isso o repo é safe pra ficar público.

**Pra adaptar pra outra empresa:** troque `cloudId` e o pageId da Home no frontmatter de [`SKILL.md`](./confluence-docs/SKILL.md), e crie a Home no seu Confluence seguindo o mesmo padrão. Veja a seção [Contribuindo](#contribuindo) pra entender por que nenhum dado de empresa específica vive aqui.

### Por que CLI em vez de só MCP

O MCP da Atlassian devolve ADF inteiro de cada página (10–40 KB de JSON). Em sessão de research + edição, queima a janela. O CLI vive em `~/.claude/skills/confluence-docs/bin/` e oferece:

- **`home --refresh`** — baixa a Home 1× por sessão e cacheia local. Queries seguintes são offline.
- **`page digest --page-id ID`** — título, versão, outline e word count em ~500 bytes.
- **`page apply --replace-section`** — edita seção atomicamente (GET → PUT com retry 409). Macros fora da seção alterada são preservadas byte-a-byte.
- **`search "termo"`** — CQL com saída TSV compacta.

Toda escrita faz GET fresh antes do PUT, então o cache nunca causa sobrescrita.

---

## Instalação

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.sh | bash
```

**Windows (PowerShell):**
```powershell
iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.ps1 | iex
```

O instalador é idempotente: baixa o último release, coloca em `~/.claude/skills/confluence-docs/`, adiciona ao PATH e reporta se já tem credencial. **Abra um terminal novo** depois (ou `source ~/.zshrc`) pra o PATH pegar.

Depois, gere um token em https://id.atlassian.com/manage-profile/security/api-tokens e configure:

```bash
confluence-docs setup                      # wizard interativo
confluence-docs setup --email X --token Y  # não-interativo (CI/agente)
confluence-docs setup --check              # valida
```

Reabra o Claude Code e pergunte: *"onde fica a página de governança?"*, *"cria uma página de parceiro novo"*, *"quais aceleradoras a Lybel está mapeando?"*.

**Atualizar:** `confluence-docs update`. **Desinstalar:** apaga `~/.claude/skills/confluence-docs/` e `~/.config/confluence-docs/`.

### Instalação assistida por IA

Cola num agente de IA qualquer:

> Quero instalar a skill `confluence-docs`. Segue o roteiro em https://github.com/lybel-app/skills/blob/main/confluence-docs/INSTALL_FOR_AI.md

O [`INSTALL_FOR_AI.md`](./confluence-docs/INSTALL_FOR_AI.md) é um runbook com exit codes determinísticos e regras de segurança pro token.

---

## Uso típico

```
Você: onde fica a página de governança?

Claude: Achei no Confluence:
- Governança Lybel — estrutura de comitês, cadência de board e RACI
  https://lybel.atlassian.net/wiki/spaces/lybel/pages/229891
```

A skill ativa automaticamente quando a pergunta bate com o escopo (busca, criação, listagem, update, status de página).

---

## Desenvolvendo

```
lybel-skills/
├── <nome-da-skill>/
│   ├── SKILL.md          # Frontmatter + instruções
│   ├── reference/        # Templates, taxonomia, workflows
│   ├── cli/              # (opcional) CLI Go que a skill usa
│   ├── install/          # (opcional) install.sh / install.ps1
│   └── bin/              # Gerado por make install — gitignored
├── .github/workflows/release.yml   # Tag v* → build cross-platform + Release
└── README.md
```

Cada skill é self-contained. Sem CLI? Pula `cli/` e `install/` — `SKILL.md` + `reference/` é o mínimo. Release assets são gerados pelo CI, nunca commitados.

## Contribuindo

Este repo é open-source e as skills aqui têm que funcionar pra qualquer empresa que clonar. Regras pra PR:

- **Skills devem ser company-agnostic.** Nenhum dado específico da Lybel (ou de qualquer empresa) hardcoded no corpo da skill, em `reference/`, ou no código do CLI. Sem nomes de pessoas, advisors, investidores, parceiros, page IDs específicos, URLs de instâncias, listas de produtos, etc.
- **Defaults configuráveis.** Se a skill precisa de um valor pra funcionar (cloudId, pageId raiz, domínio Atlassian), expõe via frontmatter ou variável de ambiente. O default pode apontar pra Lybel — mas tem que estar documentado como trocar.
- **Padrão "Home page como fonte da verdade".** Pra dados que mudam (taxonomia, índice, lista de itens), a skill deve **consultar o sistema externo em runtime** (Confluence, Jira, etc.), não cachear no repo. É isso que mantém o repo timeless e safe pra deixar público.
- **Exceções OK:** o README, CHANGELOG, e commits podem mencionar Lybel à vontade — é a empresa mantenedora. Só o conteúdo das skills é que precisa ser genérico.

Antes de abrir PR, grep no diff: `git diff main | grep -iE 'lybel|d\.clair|11C47E|164232'`. Se aparecer fora de README/CHANGELOG/configs default documentados, refatora.

### Adicionar skill nova

1. Cria `<nome>/SKILL.md` seguindo o formato de [skills.md](https://docs.claude.com/en/docs/claude-code/skills).
2. Põe templates/workflows em `<nome>/reference/`.
3. Se precisa de CLI, cria `<nome>/cli/` com `main.go` + `Makefile`.
4. Pra testar local sem reinstalar a cada mudança:
   ```bash
   ln -s "$(pwd)/<nome>" ~/.claude/skills/<nome>
   ```
   (Windows: `mklink /J`. Alguns sandboxes de IA bloqueiam symlink em `~/.claude/skills/` — copie nesse caso.)
5. PR + tag `vX.Y.Z` → CI publica release automaticamente.

### Convenções

- `name` no frontmatter: lowercase com hífens, máx 64 chars.
- `description`: máx 1024 chars, com triggers (frases que ativam a skill).
- Corpo do SKILL.md em **pt-BR**; frontmatter em inglês.
- Referências usam paths relativos (`reference/foo.md`), nunca URL absoluto.

---

## License

[MIT](./LICENSE) © 2026 Lybel
