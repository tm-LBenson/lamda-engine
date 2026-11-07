(function(root){
 'use strict';
 const limits=Object.freeze({rules:200,blocks:80,depth:8,auras:64,timers:90,measurements:64,anchors:64,combat:8,talents:16,outfits:8,displays:64,fileBytes:4*1024*1024});
 root.OverlayLimits=limits;
 if(typeof module!=='undefined')module.exports=limits;
})(typeof window!=='undefined'?window:globalThis);
