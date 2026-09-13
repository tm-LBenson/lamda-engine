local _,LUI=...
LUI:RegisterModule({id="cd",name="LamdaCD",build=function(parent)
    LUI:Checkbox(parent,"Enabled","cdEnabled",0)
    LUI:Checkbox(parent,"Companions","companions",-32)
    LUI:Checkbox(parent,"Preview","preview",-64)
    LUI:Number(parent,"Left","overlayX",-104,0,16000)
    LUI:Number(parent,"Top","overlayY",-144,0,16000)
    LUI:Number(parent,"Scale","overlayScale",-184,0.5,3)
end})
