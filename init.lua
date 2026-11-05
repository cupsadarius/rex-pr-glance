-- rex-pr-glance: PR Glance beside the focused terminal in Rex.
--
--   require("rex-pr-glance").setup{}
--   rex.bind("cmd+g", "pr_glance")
--
-- The pr_glance action toggles the pane in the focused window: it opens it to
-- the right of the focused terminal, focuses it when it is open, and closes it
-- when it already has focus. The pane follows the terminal you work in and
-- shows the pull request of that checkout's branch.

local M = {}

M.version = "0.1.0"

M.options = {
  -- The part of the split the focused terminal keeps, between 0 and 1.
  ratio = 0.65,
  -- The label that marks the pane, so the action finds it again.
  label = "pr-glance",
  -- The binary. Defaults to bin/rex-pr-glance beside this file.
  bin = nil,
}

local SHELL = "com.superlogical.terminal.shell"
local TERMINAL = "com.superlogical.terminal"

-- The directory this file is in. Rex names a module's chunk by its absolute
-- path; a relative name is taken from beside the init.lua Rex opened.
local function module_dir()
  local src = (debug.getinfo(1, "S").source or ""):gsub("^@", "")
  local dir = src:match("^(/.*)/[^/]*$")
  if dir then return dir end
  local cfg = os.getenv("REX_CONFIG")
  local base = cfg and cfg:match("^(.*)/[^/]*$")
    or (os.getenv("XDG_CONFIG_HOME") or (os.getenv("HOME") .. "/.config")) .. "/rex"
  local rel = src:match("^(.*)/[^/]*$")
  return rel and (base .. "/" .. rel) or base
end

local function expand_home(path)
  if path:sub(1, 2) == "~/" then return os.getenv("HOME") .. path:sub(2) end
  return path
end

-- The window that holds block_id, and the pane in that window, if any.
local function find(session_id, block_id)
  local view, err = rex.call("session.view", { session_id = session_id })
  if not view then error("pr-glance: " .. tostring(err), 0) end
  for _, w in ipairs(view.windows or {}) do
    local here, pane = false, nil
    for _, l in ipairs(w.layers or {}) do
      for _, b in ipairs(l.blocks or {}) do
        if b.block_id == block_id then here = true end
        if b.label == M.options.label then pane = b.block_id end
      end
    end
    if here then return w, pane end
  end
  return nil, nil
end

-- The focused terminal's directory, so the pane starts in the same checkout.
local function block_cwd(block_id)
  local p = rex.block.call(TERMINAL, "process", { block_id = block_id })
  if not p then return nil end
  local cwd = (p.foreground or {}).cwd or (p.child or {}).cwd
  if cwd and cwd ~= "" then return cwd end
  return nil
end

function M.toggle(ctx)
  if not ctx.block_id then return false end
  local window, pane = find(ctx.session_id, ctx.block_id)
  if pane and window.focused_block_id == pane then
    rex.call("block.close", { session_id = ctx.session_id, block_id = pane })
    return true
  end
  if pane then
    rex.call("session.focus_block", { session_id = ctx.session_id, block_id = pane })
    return true
  end
  local options = { command = { M.options.bin, "view", "--from", ctx.block_id } }
  options.cwd = block_cwd(ctx.block_id)
  local _, err = rex.call("session.new_split", {
    session_id = ctx.session_id,
    anchor_id = ctx.block_id,
    direction = "horizontal",
    side = "after",
    ratio = M.options.ratio,
    focus = false,
    layout = rex.layout.block({ flavor = SHELL, label = M.options.label, options = options }),
  })
  if err then error("pr-glance: " .. tostring(err), 0) end
  return true
end

function M.setup(opts)
  for k, v in pairs(opts or {}) do
    if M.options[k] ~= nil or k == "bin" then
      M.options[k] = v
    else
      rex.log("warn", "rex-pr-glance: ignoring unknown option " .. tostring(k))
    end
  end
  M.options.bin = expand_home(M.options.bin or (module_dir() .. "/bin/rex-pr-glance"))

  rex.action({
    name = "pr_glance",
    title = "PR Glance: toggle",
    description = "Open PR Glance beside the focused terminal, focus it, or close it when it has focus.",
    category = "PR Glance",
    keywords = "github pull request pr glance review ci",
    run = function(ctx) return M.toggle(ctx) end,
  })

  return M
end

return M
