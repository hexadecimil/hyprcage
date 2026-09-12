-- Minimal Lua configuration for the phase-0 VM: the same content as
-- hyprland.conf, in the Lua dialect of Hyprland >= 0.53.
hl.monitor({ output = "", mode = "preferred", position = "auto", scale = 1 })
hl.config({
    misc = { disable_hyprland_logo = true, disable_splash_rendering = true },
    cursor = { no_hardware_cursors = true },
})
for i = 1, 9 do
    hl.bind("SUPER + " .. i, hl.dsp.focus({ workspace = i }))
end
hl.bind("SUPER + Q", hl.dsp.window.close())
