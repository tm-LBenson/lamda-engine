local _,LUI=...
LUI:RegisterModule({id="cd",name="LamdaCD",build=function(parent)
    LUI:Checkbox(parent,"Enabled","cdEnabled",0)
    LUI:Number(parent,"Left","overlayX",-48,0,16000)
    LUI:Number(parent,"Top","overlayY",-88,0,16000)
    LUI:Number(parent,"Scale","overlayScale",-128,0.5,3)
end})
