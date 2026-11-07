local _,LUI=...
local K=LUI.KBB
local U={page="board",columnPage=1,offsets={},category="All categories",archived=false,fieldType="Text"}
K.UI=U
local backdrop={bgFile="Interface\\Buttons\\WHITE8X8",edgeFile="Interface\\Buttons\\WHITE8X8",edgeSize=1}
-- gsub also returns a replacement count; widget APIs must receive only text.
local function display(value)return (tostring(value or ""):gsub("|","||"))end
local noteBullet="|TInterface\\AddOns\\LamdaFrames\\Libs\\DandersUI\\Media\\Icons\\dot.tga:6:6:0:0|t "
local function displayNotes(value,limit)
    -- Escape the user's text before adding our own trusted texture. Keep the
    -- saved note/editor plain text, and truncate before inserting any markup.
    local text=display(tostring(value or ""):sub(1,limit or 600)):gsub("\r\n","\n"):gsub("\r","\n")
    return (text:gsub("[^\n]+",function(line)
        local indent,body=line:match("^([ \t]*)%-[ \t]+(.*)$")
        if indent then return indent..noteBullet..body end
        return line
    end))
end
local function box(parent,x,y,w,h)
    local f=CreateFrame("Frame",nil,parent,"BackdropTemplate");f:SetPoint("TOPLEFT",x,y);f:SetSize(w,h)
    f:SetBackdrop(backdrop);f:SetBackdropColor(.035,.065,.09,.98);f:SetBackdropBorderColor(.18,.29,.36,1);return f
end
local function label(parent,value,x,y,w)
    return LUI:Description(parent,value,x,y,w)
end
local function input(parent,x,y,w,h,max,multiline)
    local panel=box(parent,x,y,w,h);local edit=CreateFrame("EditBox",nil,panel)
    edit:SetPoint("TOPLEFT",8,-5);edit:SetPoint("BOTTOMRIGHT",-8,5);edit:SetFontObject("GameFontHighlight")
    edit:SetAutoFocus(false);edit:SetMaxLetters(max);edit:SetMultiLine(not not multiline)
    edit:SetScript("OnEscapePressed",function(self)self:ClearFocus()end)
    if not multiline then edit:SetScript("OnEnterPressed",function(self)self:ClearFocus()end)end
    edit.panel=panel;return edit
end
local function check(parent,title,x,y,callback)
    local c=CreateFrame("CheckButton",nil,parent,"UICheckButtonTemplate");c:SetPoint("TOPLEFT",x,y);c:SetSize(26,26)
    local t=label(c,title,32,-5,530);c.caption=t
    c:SetScript("OnClick",function(self)callback(not not self:GetChecked())end);return c
end
-- Values are IDs, labels are only presentation. Renames never break references.
local function select(parent,x,y,w,items,get,set)
    local b=LUI:Dropdown(parent,x,y,w,function()
        local names={};for _,item in ipairs(items())do names[#names+1]=display(item.label)end;return names
    end,function()
        for _,item in ipairs(items())do if item.value==get()then return display(item.label)end end;return "Choose…"
    end,function(name)
        for _,item in ipairs(items())do if display(item.label)==name then set(item.value);break end end
    end)
    function b:UpdateLabel()
        for _,item in ipairs(items())do if item.value==get()then self:SetText(display(item.label));return end end;self:SetText("Choose…")
    end
    return b
end
local function options(values)local rows={};for _,v in ipairs(values)do rows[#rows+1]={value=v,label=v}end;return rows end
function U:Columns(exclude)
    local b=K:Board();local rows={}
    for _,c in ipairs(b and b.columns or {})do if c.id~=exclude then rows[#rows+1]={value=c.id,label=c.name}end end;return rows
end
function U:Message(message)if self.message then self.message:SetText(display(message or ""))end end
function U:BoardChanged()
    self.dragID=nil;self.columnPage=1;self.offsets={};self.category="All categories";self.archived=false
    self.selectedField=nil;self.selectedQuest=nil
    if LUI.openMenu then LUI.openMenu:Hide()end
    if self.confirm then self.confirm:Hide()end
    if self.boardDialog then self.boardDialog:Hide()end
    if self.displayDialog then self.displayDialog:Hide()end
    for _,f in pairs(self.displays or {})do self:StopDisplayResize(f,false);self:StopDisplayDrag(f,false);f:Hide()end
    if not self.frame then return end
    self.search:SetText("");self.categorySelect:UpdateLabel();self.archiveSelect:UpdateLabel()
    self:ChooseColumn(K:Board().columns[1].id)
    for _,edit in ipairs({self.newColumn,self.newField,self.fieldOptions,self.fieldName})do edit:SetText("")end
    self.hideField:SetChecked(false)
    if K:Board().fields[1]then self:ChooseField(K:Board().fields[1].id)end
    if self.page=="quests"then self:LoadQuests()end
    self:Message("")
end
function U:Result(ok,why,success)
    self:Message(why or (ok and success) or "The change could not be saved.")
    if ok then self:Refresh()end;return ok
end
function U:Confirm(message,action)
    if not self.confirm then
        local f=box(self.frame,270,-230,500,170);self.confirm=f;f:SetFrameLevel(self.frame:GetFrameLevel()+40);f:EnableMouse(true)
        f.prompt=label(f,"",18,-18,464)
        LUI:Button(f,"Confirm",18,-125,150,function()local fn=f.action;f:Hide();fn()end)
        LUI:Button(f,"Cancel",330,-125,150,function()f:Hide()end)
    end
    self.confirm.prompt:SetText(display(message));self.confirm.action=action;self.confirm:Show()
end
function U:Refresh()
    self:RefreshDisplays()
    if not self.frame or not self.frame:IsShown()then return end
    if not LUI:DB().kbbEnabled then self.frame:Hide();return end
    local b,err=K:Board();if not b then self:Message(err);return end
    self.boardSelect:UpdateLabel()
    if self.page=="board" then self:DrawBoard()end
    if self.editor and self.editor:IsShown()then self:RefreshEditorQuest()end
    self.columnSelect:UpdateLabel();self.destinationSelect:UpdateLabel();self.fieldSelect:UpdateLabel()
    self.questColumn:UpdateLabel();self.doneColumn:UpdateLabel();self.autoWatch:SetChecked(b.autoWatch);self.autoDone:SetChecked(b.autoDone)
    self.questPick:UpdateLabel()
    self:RefreshDisplayControls()
end
function U:Open()
    if not LUI:DB().kbbEnabled then
        LUI:OpenUI();LUI:SelectPage("modules");LUI:SelectModule("kbb");return
    end
    local b,err=K:Board(true)
    if not b then if DEFAULT_CHAT_FRAME then DEFAULT_CHAT_FRAME:AddMessage("LamdaKBB: "..(err or "Board unavailable"))end;return end
    if not self.frame then self:Build()end
    self.frame:SetScale(math.min(1,UIParent:GetWidth()*.94/1040,UIParent:GetHeight()*.94/700))
    self.frame:Show();K:RefreshQuests();self:Refresh()
end
function U:SelectPage(page)
    if LUI.openMenu then LUI.openMenu:Hide()end
    self.page=page
    for key,panel in pairs(self.pages)do panel:SetShown(key==page);self.tabs[key]:SetSelected(key==page)end
    if page=="quests" then self:LoadQuests()end
    self:Message("");self:Refresh()
end
function U:BuildSettings(parent)
    LUI:Button(parent,"Open planner",4,0,190,function()LUI.frame:Hide();U:Open()end)
    label(parent,"Personal boards for this character",4,-55,600)
    label(parent,"Plan dungeon farms, achievements, recipes, rotations, or anything else. Create cards, move them between your own columns, and add custom fields.",4,-88,580)
    label(parent,"Open with /lamda kbb. Switch boards using the top dropdown, or create a new plan. Export and import boards to share steps, notes, and custom fields. Columns, Fields, and Quests customize the selected board.",4,-158,580)
    label(parent,"Cards and board structure save independently of cooldown profiles. Restoring LamdaUI settings keeps your plans. Changes take effect immediately; WoW writes saved data when you reload or log out.",4,-238,580)
    self.hideTrackerSettings=check(parent,"Hide WoW’s quest/objective tracker",4,-319,function(v)local ok,err=K:SetHideQuestTracker(v);U:Result(ok,err)end)
    LUI:Explain(self.hideTrackerSettings,"Hide WoW’s quest/objective tracker","Hide the on-screen tracker, including scenario, achievement and other objectives. Your quest log and KBB quest syncing still work. Turn this off or disable LamdaKBB to restore it. Changes during combat apply after combat.")
    self.trackerSettingsStatus=label(parent,"",4,-363,580)
    label(parent,"Lamda Engine > Planner manages these boards on your desktop. In-game changes send automatically while WoW is in front. Desktop changes load on your next /reload.",4,-410,580)
    self.syncStatus=label(parent,"",4,-480,580)
    LUI:OnRefresh(parent,function()
        U:RefreshDisplays();U:RefreshDisplayControls()
        if U.syncStatus then U.syncStatus:SetText(K.Sync and K.Sync.error and ("Planner sync: "..K.Sync.error)or "Retail: 127.0.0.1:47641/kbb  |  Forever: 127.0.0.1:47642/kbb")end
        if U.frame and not LUI:DB().kbbEnabled then U.frame:Hide()end
    end)
end
function U:Build()
    local f=CreateFrame("Frame","LamdaKBBBoard",UIParent,"BackdropTemplate");self.frame=f;f:Hide()
    f:SetSize(1040,700);f:SetPoint("CENTER");f:SetFrameStrata("DIALOG");f:SetClampedToScreen(true);f:EnableMouse(true)
    f:SetBackdrop(backdrop);f:SetBackdropColor(.025,.045,.06,.99);f:SetBackdropBorderColor(.16,.65,.76,1)
    f:SetMovable(true);f:RegisterForDrag("LeftButton")
    f:SetScript("OnDragStart",function(self)self:StartMoving()end)
    f:SetScript("OnDragStop",function(self)self:StopMovingOrSizing()end)
    f:SetScript("OnHide",function()U.dragID=nil;if LUI.openMenu then LUI.openMenu:Hide()end end)
    UISpecialFrames[#UISpecialFrames+1]="LamdaKBBBoard"
    label(f,"LamdaKBB  /  Planner",18,-16,275)
    label(f,"Board",318,-18,60)
    self.boardSelect=select(f,375,-12,297,function()
        local rows={};for _,b in ipairs(K:Collection().boards)do rows[#rows+1]={value=b.id,label=b.name}end;return rows
    end,function()return K:Board().id end,function(v)local ok,err=K:SelectBoard(v);U:Result(ok,err)end)
    LUI:Button(f,"New board",683,-12,115,function()U:OpenBoardDialog("new")end)
    LUI:Button(f,"Rename",807,-12,114,function()U:OpenBoardDialog("rename")end)
    label(f,"Each board has its own columns, cards, fields, and on-screen layout. Switch plans or share one below.",18,-47,1000)
    LUI:Button(f,"Close",930,-12,92,function()f:Hide()end)
    self.tabs={};self.pages={}
    for i,def in ipairs({{"board","Board"},{"columns","Columns"},{"fields","Fields"},{"quests","Quests"}})do
        local page=CreateFrame("Frame",nil,f);page:SetPoint("TOPLEFT",16,-116);page:SetSize(1008,530);page:Hide();self.pages[def[1]]=page
        local key=def[1];self.tabs[key]=LUI:Navigation(f,def[2],16+(i-1)*145,-76,137,function()U:SelectPage(key)end)
    end
    self.message=label(f,"",18,-657,1000)
    LUI:Button(f,"Import board",708,-76,148,function()U:OpenBoardDialog("import")end)
    LUI:Button(f,"Export board",866,-76,156,function()U:OpenBoardDialog("export")end)
    self:BuildBoard(self.pages.board);self:BuildColumns(self.pages.columns);self:BuildFields(self.pages.fields);self:BuildQuests(self.pages.quests)
    self:BuildEditor();self:SelectPage("board")
end
function U:OpenBoardDialog(mode)
    local ok,err=K:CanSwitchBoard();if not ok then self:Message(err);return end
    if LUI.openMenu then LUI.openMenu:Hide()end
    local exported
    if mode=="export"then exported,err=K:ExportBoard();if not exported then self:Message(err);return end end
    if not self.boardDialog then
        local shield=CreateFrame("Frame",nil,self.frame);shield:SetAllPoints(self.frame);shield:EnableMouse(true)
        shield:SetFrameLevel(self.frame:GetFrameLevel()+42);shield:Hide()
        local d=box(self.frame,100,-90,840,510);self.boardDialog=d;d:EnableMouse(true);d:SetFrameLevel(shield:GetFrameLevel()+2);d:Hide()
        d:SetScript("OnShow",function()shield:Show()end)
        d:SetScript("OnHide",function()shield:Hide();d.name:ClearFocus();d.text:ClearFocus()end)
        d.title=label(d,"",18,-18,804);d.help=label(d,"",18,-51,804)
        d.name=input(d,18,-105,804,30,60)
        d.panel=box(d,18,-112,804,284)
        d.scroll=CreateFrame("ScrollFrame",nil,d.panel,"UIPanelScrollFrameTemplate");d.scroll:SetPoint("TOPLEFT",8,-8);d.scroll:SetSize(758,268)
        d.text=CreateFrame("EditBox",nil,d.scroll);d.text:SetSize(746,268);d.text:SetMultiLine(true);d.text:SetAutoFocus(false)
        d.text:SetMaxLetters(K.shareLimit+4097);d.text:SetFontObject("GameFontHighlightSmall");d.scroll:SetScrollChild(d.text);d.text:SetPoint("TOPLEFT")
        local function focus(_,button)if button=="LeftButton"then d.text:SetFocus()end end
        d.panel:EnableMouse(true);d.panel:SetScript("OnMouseDown",focus);d.scroll:EnableMouse(true);d.scroll:SetScript("OnMouseDown",focus)
        d.text:SetScript("OnTextChanged",function()d.scroll:SetVerticalScroll(0)end)
        d.text:SetScript("OnEscapePressed",function(self)self:ClearFocus()end)
        d.error=label(d,"",18,-410,804);d.error:SetTextColor(1,.65,.3)
        d.action=LUI:Button(d,"",18,-459,250,function()U:SubmitBoardDialog()end)
        d.cancel=LUI:Button(d,"Cancel",644,-459,178,function()d:Hide()end)
        d.name:SetScript("OnEnterPressed",function()U:SubmitBoardDialog()end)
    end
    local d=self.boardDialog;d.mode=mode;d.boardID=K:Board().id;d.character=K:CharacterKey();d.error:SetText("")
    local share=mode=="import"or mode=="export"
    d:SetHeight(share and 510 or 235);d.panel:SetShown(share);d.name.panel:SetShown(not share)
    d.action:ClearAllPoints();d.action:SetPoint("BOTTOMLEFT",18,25);d.cancel:ClearAllPoints();d.cancel:SetPoint("BOTTOMRIGHT",-18,25)
    d.error:ClearAllPoints();d.error:SetPoint("TOPLEFT",18,share and -410 or -146)
    local titles={new="New board",rename="Rename board",import="Import board",export="Export board"};d.title:SetText(titles[mode])
    local help={new="Give this plan a name. It starts with Backlog, Next, Doing, and Done.",rename="Choose a name for this board. Its cards, fields, and positions stay the same.",
        import="Paste the entire export string (Ctrl+V). Import creates a separate board and keeps your existing plans. Display and automatic quest options start off.",
        export="Copy the selected string (Ctrl+C) to share this board, including notes, custom fields, archived cards, and quest links. Your screen settings and character identity are excluded."}
    d.help:SetText(help[mode]);d.action:SetText(mode=="export"and "Select all for copying"or mode=="import"and "Import as new board"or mode=="new"and "Create board"or "Save name")
    d.cancel:SetText(mode=="export"and "Done"or "Cancel")
    d.name:SetText(mode=="rename"and K:Board().name or "");d.text:SetText(exported or "");d:Show()
    if share then d.text:SetFocus();d.text:HighlightText();d.scroll:SetVerticalScroll(0)else d.name:SetFocus();d.name:HighlightText()end
end
function U:SubmitBoardDialog()
    local d=self.boardDialog;local b=K:Board()
    if d.character~=K:CharacterKey()or not b or d.boardID~=b.id then d.error:SetText("The selected board changed. Close this dialog and try again.");return end
    if d.mode=="export"then d.text:SetFocus();d.text:HighlightText();return end
    local ok,err
    if d.mode=="new"then ok,err=K:NewBoard(d.name:GetText())
    elseif d.mode=="rename"then ok,err=K:RenameBoard(d.name:GetText())
    else ok,err=K:ImportBoard(d.text:GetText())end
    if not ok then d.error:SetText(display(err));return end
    d:Hide();self:Refresh();self:Message(d.mode=="import"and "Board imported. Your other boards are unchanged."or "Board saved.")
end
function U:BuildBoard(p)
    label(p,"Search titles, notes, and custom fields",0,0,340)
    self.search=input(p,0,-23,310,28,120)
    self.search:SetScript("OnTextChanged",function()U.offsets={};U:DrawBoard()end)
    self.categorySelect=select(p,322,-23,220,function()local v={"All categories"};for _,k in ipairs(K.kinds)do v[#v+1]=k end;return options(v)end,
        function()return U.category end,function(v)U.category=v;U.offsets={};U:DrawBoard()end)
    self.archiveSelect=select(p,554,-23,210,function()return {{value=false,label="Active cards"},{value=true,label="Archived cards"}}end,
        function()return U.archived end,function(v)U.archived=v;U.offsets={};U:DrawBoard()end)
    LUI:Button(p,"+ New card",786,-23,218,function()U:EditCard()end)
    self.lanes={}
    for lane=1,4 do
        local panel=box(p,(lane-1)*254,-70,246,411);self.lanes[lane]=panel;panel.rows={}
        panel.heading=label(panel,"",10,-12,190);panel.heading:SetMaxLines(1)
        panel.add=LUI:Button(panel,"+",208,-5,28,function()U:EditCard(nil,panel.columnID)end)
        panel.display=check(panel,"Display on UI",6,-34,function(v)K:SetColumnDisplay(panel.columnID,"showOnUI",v)end);panel.display.caption:SetWidth(115)
        panel.lock=check(panel,"Lock",162,-34,function(v)K:SetColumnDisplay(panel.columnID,"uiLocked",v)end);panel.lock.caption:SetWidth(47)
        LUI:Explain(panel.display,"Display on UI","Show this column on your game screen, even with the planner closed. Drag its header while unlocked to position it.")
        LUI:Explain(panel.lock,"Lock position","Prevent moving this displayed column. Its cards remain clickable and scrollable.")
        panel:SetScript("OnMouseUp",function()U:Drop(panel.columnID)end);panel:EnableMouse(true)
        for row=1,4 do
            local button=CreateFrame("Button",nil,panel,"BackdropTemplate");panel.rows[row]=button
            button:SetPoint("TOPLEFT",8,-73-(row-1)*68);button:SetSize(230,62);button:SetBackdrop(backdrop)
            button:SetBackdropColor(.075,.12,.16,1);button:SetBackdropBorderColor(.22,.32,.39,1)
            button.title=label(button,"",9,-7,212);button.title:SetMaxLines(2)
            button.detail=label(button,"",9,-41,212);button.detail:SetMaxLines(1);button.detail:SetTextColor(.55,.85,.91)
            button:RegisterForDrag("LeftButton")
            button:SetScript("OnDragStart",function()if not U.archived then U.dragID=button.cardID;button.dragged=true;button:SetAlpha(.45);U:Message("Drop onto a column or before another card. Release outside the board to cancel.")end end)
            button:SetScript("OnDragStop",function()
                button:SetAlpha(1)
                C_Timer.After(0,function()button.dragged=false end)
                if U.dragID then
                    local column,before
                    for _,target in ipairs(U.lanes)do if target:IsShown() and target:IsMouseOver()then
                        column=target.columnID
                        for _,r in ipairs(target.rows)do if r:IsShown() and r:IsMouseOver()then before=r.cardID;break end end
                        break
                    end end
                    U:Drop(column,before)
                end
            end)
            button:SetScript("OnClick",function()if button.dragged then return end;if U.dragID then U:Drop(panel.columnID,button.cardID)else U:EditCard(button.cardID)end end)
            button:SetScript("OnEnter",function()
                if not GameTooltip or U.dragID then return end
                local card=button.card;if not card then return end
                GameTooltip:SetOwner(button,"ANCHOR_RIGHT");GameTooltip:SetText(display(card.title))
                GameTooltip:AddLine(display(card.priority.." · "..card.kind),.55,.85,.91)
                if card.questID then GameTooltip:AddLine(display((card.questStatus or "Quest").."\n"..(card.questProgress or "")),.85,.9,.95,true)end
                if card.notes~="" then GameTooltip:AddLine(displayNotes(card.notes,600),.9,.92,.95,true)end
                for _,field in ipairs(K:Board().fields)do if not field.hidden and card.values[field.id]~=nil then
                    GameTooltip:AddLine(display(field.name..": "..tostring(card.values[field.id])),.75,.85,.9,true)
                end end
                GameTooltip:AddLine("Click to edit. Drag to move.",.55,.85,.91);GameTooltip:Show()
            end)
            local function hideTip()if GameTooltip then GameTooltip:Hide()end end
            button:SetScript("OnLeave",hideTip);button:SetScript("OnHide",hideTip)
        end
        panel.empty=label(panel,"No cards here. Use + to add one.",12,-89,222)
        panel.count=label(panel,"",50,-368,144)
        LUI:Button(panel,"<",8,-357,32,function()local key=panel.columnID;U.offsets[key]=math.max(0,(U.offsets[key]or 0)-4);U:DrawBoard()end)
        LUI:Button(panel,">",206,-357,32,function()local key=panel.columnID;U.offsets[key]=(U.offsets[key]or 0)+4;U:DrawBoard()end)
        panel:EnableMouseWheel(true);panel:SetScript("OnMouseWheel",function(_,delta)local key=panel.columnID;U.offsets[key]=math.max(0,(U.offsets[key]or 0)-delta);U:DrawBoard()end)
    end
    self.previous=LUI:Button(p,"< Earlier columns",0,-496,180,function()U.columnPage=U.columnPage-1;U:DrawBoard()end)
    self.pageLabel=label(p,"",198,-502,600)
    self.next=LUI:Button(p,"Later columns >",824,-496,180,function()U.columnPage=U.columnPage+1;U:DrawBoard()end)
end
function U:DrawBoard()
    if not self.lanes then return end
    local b=K:Board();if not b then return end
    local max=math.ceil(#b.columns/4);self.columnPage=math.max(1,math.min(self.columnPage,max))
    for i,panel in ipairs(self.lanes)do
        local c=b.columns[(self.columnPage-1)*4+i];panel:SetShown(c~=nil)
        if c then
            panel.columnID=c.id
            panel.display:SetChecked(c.showOnUI==true);panel.lock:SetChecked(c.uiLocked==true)
            local rows=K:Cards(c.id,self.search:GetText(),self.category,self.archived)
            local offset=math.max(0,math.min(self.offsets[c.id]or 0,math.max(0,#rows-4)));self.offsets[c.id]=offset
            panel.heading:SetText(display(c.name).." ("..#rows..")");panel.empty:SetShown(#rows==0)
            panel.count:SetText(#rows==0 and "0 cards" or ((offset+1).."–"..math.min(offset+4,#rows).." of "..#rows))
            for j,button in ipairs(panel.rows)do
                local card=rows[offset+j];button:SetShown(card~=nil)
                if card then
                    button.cardID=card.id;button.card=card;button.title:SetText(display(card.title));button:SetAlpha(1)
                    local detail=card.questID and (card.questStatus or "Quest") or (card.priority~="Normal" and card.priority.." · "..card.kind or card.kind)
                    if card.questID and card.questStatus=="In progress" and card.questProgress~="" then detail=card.questProgress or detail end
                    button.detail:SetText(display(detail))
                    button:SetBackdropBorderColor(card.priority=="High" and .9 or .22,card.priority=="High" and .65 or .32,.39,1)
                end
            end
        end
    end
    self.previous:SetEnabled(self.columnPage>1);self.next:SetEnabled(self.columnPage<max)
    self.pageLabel:SetText("Columns "..((self.columnPage-1)*4+1).."–"..math.min(self.columnPage*4,#b.columns).." of "..#b.columns.." · Scroll each column to see more cards")
end
function U:Drop(column,before)
    local card=self.dragID;self.dragID=nil
    if not card then return end
    if card and column then local ok,err=K:MoveCard(card,column,before);self:Result(ok,err,"Card moved.")else self:Message("Move cancelled.");self:DrawBoard()end
end
function U:BuildColumns(p)
    label(p,"Make the workflow your own. Columns are ordered left to right.",0,0,900)
    label(p,"New column",0,-44,400);self.newColumn=input(p,0,-67,380,30,40)
    LUI:Button(p,"Add column",400,-68,180,function()
        local ok,err=K:AddColumn(U.newColumn:GetText());if U:Result(ok,err,"Column added.")then U.newColumn:SetText("")end
    end)
    label(p,"Edit column",0,-127,400)
    self.columnSelect=select(p,0,-151,380,function()return U:Columns()end,function()return U.selectedColumn end,function(v)U:ChooseColumn(v)end)
    self.columnName=input(p,0,-194,380,30,40)
    LUI:Button(p,"Rename",400,-194,180,function()local ok,err=K:RenameColumn(U.selectedColumn,U.columnName:GetText());U:Result(ok,err,"Column renamed.")end)
    LUI:Button(p,"Move left",0,-240,180,function()local ok,err=K:ReorderColumn(U.selectedColumn,-1);U:Result(ok,err,"Column moved.")end)
    LUI:Button(p,"Move right",200,-240,180,function()local ok,err=K:ReorderColumn(U.selectedColumn,1);U:Result(ok,err,"Column moved.")end)
    label(p,"Remove a column and move all its cards to:",0,-315,800)
    self.destinationSelect=select(p,0,-347,380,function()return U:Columns(U.selectedColumn)end,function()return U.columnDestination end,function(v)U.columnDestination=v end)
    LUI:Button(p,"Remove column…",400,-347,180,function()
        local from,to=U.selectedColumn,U.columnDestination
        if not from or not to or from==to then U:Message("Choose a column and a different destination.");return end
        U:Confirm("Remove this column? All its active and archived cards move to the chosen destination. Their notes and fields are kept.",function()
            local ok,err=K:RemoveColumn(from,to);if ok then U:ChooseColumn(to)end;U:Result(ok,err,"Column removed; cards moved.")
        end)
    end)
    label(p,"Removing a column never deletes its cards. Quest import and completion destinations follow the cards to the replacement column.",0,-409,580)
    label(p,"Selected column on your game screen",620,-127,370)
    self.columnDisplay=check(p,"Display on UI",620,-171,function(v)local ok,err=K:SetColumnDisplay(U.selectedColumn,"showOnUI",v);U:Result(ok,err)end)
    self.columnDisplay.caption:SetWidth(325)
    LUI:Explain(self.columnDisplay,"Display on UI","Show this column's active cards while the planner is closed. Each column has its own position. Scroll the displayed list to see more cards; click a card to edit it.")
    self.columnLock=check(p,"Lock position",620,-212,function(v)local ok,err=K:SetColumnDisplay(U.selectedColumn,"uiLocked",v);U:Result(ok,err)end)
    self.columnLock.caption:SetWidth(325)
    LUI:Explain(self.columnLock,"Lock position","Prevent this column from being dragged. Cards remain clickable and scrollable. Unlock and drag the column header to move it.")
    self.columnTransparent=check(p,"Transparent background",620,-253,function(v)local ok,err=K:SetColumnDisplay(U.selectedColumn,"uiTransparent",v);U:Result(ok,err)end)
    self.columnTransparent.caption:SetWidth(325)
    LUI:Explain(self.columnTransparent,"Transparent background","Show a bullet list over the game with outlined titles and indented details. Hover the list to reveal Planner, paging, lock, minimize, and resize controls. Cards remain clickable and scrollable. Applies immediately to this column; the planner board keeps its usual style.")
    LUI:Button(p,"Reset position",620,-300,190,function()local ok,err=K:SetColumnPosition(U.selectedColumn);U:Result(ok,err,"Column position reset.")end)
    LUI:Button(p,"Display settings…",620,-342,260,function()U:OpenDisplaySettings(U.selectedColumn)end)
    label(p,"Choose where and when this list appears, its size, and a compact quest-tracker style. Unlock to move or resize it; use − to minimize.",620,-390,360)
    self:ChooseColumn(K:Board().columns[1].id)
end
function U:ChooseColumn(value)
    self.selectedColumn=value;self.columnDestination=nil
    for _,c in ipairs(K:Board().columns)do if c.id==value then self.columnName:SetText(c.name)elseif not self.columnDestination then self.columnDestination=c.id end end
    self:RefreshDisplayControls()
end
function U:BuildFields(p)
    label(p,"Custom fields appear on every card. Examples: target item, key level, spec, weekly goal, or a checklist flag.",0,0,920)
    label(p,"New field name",0,-54,360);self.newField=input(p,0,-77,370,30,40)
    label(p,"Type",390,-54,200)
    select(p,390,-78,200,function()return options({"Text","Number","Checkbox","Choice"})end,function()return U.fieldType end,function(v)
        U.fieldType=v;U.fieldOptions.panel:SetShown(v=="Choice");U.fieldOptionsLabel:SetShown(v=="Choice")
    end)
    self.fieldOptionsLabel=label(p,"Choices, separated by commas (2–12)",0,-124,700)
    self.fieldOptions=input(p,0,-151,590,30,800);self.fieldOptions.panel:Hide();self.fieldOptionsLabel:Hide()
    LUI:Button(p,"Add field",620,-78,180,function()
        local ok,err=K:AddField(U.newField:GetText(),U.fieldType,U.fieldOptions:GetText())
        if U:Result(ok,err,"Field added. Open any card to enter a value.")then U.newField:SetText("");U.fieldOptions:SetText("");U:ChooseField(ok.id)end
    end)
    label(p,"Edit or hide a field",0,-228,800)
    self.fieldSelect=select(p,0,-260,370,function()
        local rows={};for _,field in ipairs(K:Board().fields)do rows[#rows+1]={value=field.id,label=field.name..(field.hidden and " (hidden)" or "").." · "..field.type}end;return rows
    end,function()return U.selectedField end,function(v)U:ChooseField(v)end)
    self.fieldName=input(p,0,-302,370,30,40)
    self.hideField=check(p,"Hide this field (keep its existing values)",0,-349,function()end)
    LUI:Button(p,"Save field",390,-302,200,function()
        local ok,err=K:UpdateField(U.selectedField,U.fieldName:GetText(),U.hideField:GetChecked());U:Result(ok,err,"Field saved.")
    end)
    label(p,"Hidden fields can be restored here. Their values stay attached to the cards. Field types and dropdown choices are fixed after creation so existing entries keep their meaning; create another field for a different structure.",0,-411,880)
    local b=K:Board();if b.fields[1]then self:ChooseField(b.fields[1].id)end
end
function U:ChooseField(value)
    self.selectedField=value
    for _,field in ipairs(K:Board().fields)do if field.id==value then self.fieldName:SetText(field.name);self.hideField:SetChecked(field.hidden);break end end
end
function U:LoadQuests()
    self.quests=K:QuestList();self.questPick:UpdateLabel()
    self.questInfo:SetText(K:QuestAvailable() and (#self.quests.." active quests available. Expand quest-log headers if a quest is missing. Watched quests are included even under collapsed headers.") or "Quest integration is unavailable in this client. Manual cards still work.")
end
function U:BuildQuests(p)
    label(p,"Bring your quest tracker into the board. Linked cards show objective progress; your notes stay yours.",0,0,940)
    label(p,"Add quests to column",0,-49,480)
    self.questColumn=select(p,0,-74,380,function()return U:Columns()end,function()return K:Board().questColumn end,function(v)K:Board().questColumn=v end)
    self.autoWatch=check(p,"Automatically add watched quests",0,-120,function(v)K:Board().autoWatch=v;if v then K:ImportWatched();K:Changed()end end)
    LUI:Explain(self.autoWatch,"Watched quest cards","When enabled, quests on this character’s quest tracker become cards in the chosen column. Existing cards, including archived ones, are reused. Unwatching a quest does not delete or archive its card.")
    LUI:Button(p,"Import watched now",420,-74,215,function()U:Message(K:ImportWatched().." new quest cards added. Existing cards kept.")end)
    self.questPick=select(p,0,-181,635,function()
        local rows={};for _,q in ipairs(U.quests or {})do rows[#rows+1]={value=q.id,label=q.title.." (#"..q.id..")"}end;return rows
    end,function()return U.selectedQuest end,function(v)U.selectedQuest=v end)
    LUI:Button(p,"Add selected quest",655,-181,220,function()
        local c,err=K:AddQuest(U.selectedQuest);U:Result(c,err,"Quest added. Its progress updates automatically.")
    end)
    LUI:Button(p,"Refresh quest list",0,-224,215,function()U:LoadQuests();K:RefreshQuests();U:Message("Quest list refreshed.")end)
    self.questInfo=label(p,"",0,-269,940)
    self.autoDone=check(p,"Move quest cards after turning in",0,-344,function(v)K:Board().autoDone=v end)
    label(p,"Completed quest destination",0,-391,480)
    self.doneColumn=select(p,0,-420,380,function()return U:Columns()end,function()return K:Board().doneColumn end,function(v)K:Board().doneColumn=v end)
    label(p,"Both automatic options start off. Without them, quest progress updates but cards stay in your chosen column. Abandoned quests keep their cards and are labeled Not in quest log.",0,-469,960)
    self.hideTracker=check(p,"Hide WoW’s quest/objective tracker",420,-344,function(v)local ok,err=K:SetHideQuestTracker(v);U:Result(ok,err)end)
    self.hideTracker.caption:SetWidth(520)
    LUI:Explain(self.hideTracker,"Hide WoW’s quest/objective tracker","Hide the whole on-screen objective tracker, including scenario and achievement objectives. Quest syncing stays on. Use Columns → Display on UI for your own tracker. Changes during combat apply after combat.")
    self.trackerStatus=label(p,"",420,-392,540)
end
function U:BuildEditor()
    -- Block clicks on underlying cards/settings while a draft is being edited.
    local shield=CreateFrame("Frame",nil,self.frame);self.editorShield=shield
    shield:SetAllPoints(self.frame);shield:SetFrameLevel(self.frame:GetFrameLevel()+18);shield:EnableMouse(true);shield:Hide()
    local e=box(self.frame,65,-30,910,622);self.editor=e;e:Hide();e:EnableMouse(true);e:SetFrameLevel(self.frame:GetFrameLevel()+20)
    e:HookScript("OnShow",function()shield:Show()end);e:HookScript("OnHide",function()shield:Hide()end)
    label(e,"Card details",18,-15,800)
    label(e,"Title",18,-49,850);e.title=input(e,18,-72,872,30,160)
    label(e,"Column",18,-116,290);label(e,"Category",328,-116,250);label(e,"Priority",638,-116,220)
    e.column=select(e,18,-139,290,function()return U:Columns()end,function()return U.draft and U.draft.column end,function(v)U.draft.column=v end)
    e.kind=select(e,328,-139,290,function()return options(K.kinds)end,function()return U.draft and U.draft.kind end,function(v)U.draft.kind=v end)
    e.priority=select(e,638,-139,252,function()return options(K.priorities)end,function()return U.draft and U.draft.priority end,function(v)U.draft.priority=v end)
    local scroll=CreateFrame("ScrollFrame",nil,e,"UIPanelScrollFrameTemplate");scroll:SetPoint("TOPLEFT",18,-185);scroll:SetSize(843,332);e.scroll=scroll
    local body=CreateFrame("Frame",nil,scroll);body:SetSize(833,950);scroll:SetScrollChild(body);e.body=body
    label(body,"Notes — start a line with - and a space for a bullet",0,0,810)
    -- A separate scrolling edit box keeps long notes inside their own viewport.
    local notesBox=box(body,0,-24,824,143)
    local notesScroll=CreateFrame("ScrollFrame",nil,notesBox,"UIPanelScrollFrameTemplate");notesScroll:SetPoint("TOPLEFT",8,-8);notesScroll:SetSize(778,126)
    local notes=CreateFrame("EditBox",nil,notesScroll);e.notes=notes
    notes:SetSize(766,126);notes:SetMultiLine(true);notes:SetAutoFocus(false);notes:SetMaxLetters(8000);notes:SetFontObject("GameFontHighlight")
    notesScroll:SetScrollChild(notes);notes:SetPoint("TOPLEFT");notes:EnableMouse(true)
    local hint=label(notesScroll,"Click here to add notes. Enter starts a new line.",0,0,756);hint:SetTextColor(.6,.7,.75)
    -- A multiline edit box can shrink to one line when empty. Make its whole
    -- viewport (including the padding) clickable, as Blizzard's input scroll does.
    local function focusNotes(_,button)if button=="LeftButton" then notes:SetFocus()end end
    notesBox:EnableMouse(true);notesBox:SetScript("OnMouseDown",focusNotes)
    notesScroll:EnableMouse(true);notesScroll:SetScript("OnMouseDown",focusNotes)
    -- Text layout can settle after OnCursorChanged. Do one deferred adjustment;
    -- do not pull the view back to the cursor while the player scrolls to read.
    local cursorY,cursorHeight=0,0
    local function updateScroll(self)
        self:SetScript("OnUpdate",nil)
        notesScroll:UpdateScrollChildRect()
        local range=math.max(0,notesScroll:GetVerticalScrollRange())
        local offset=math.max(0,math.min(range,notesScroll:GetVerticalScroll()))
        if self:HasFocus()then
            if cursorY<offset then offset=cursorY
            elseif cursorY+cursorHeight>offset+notesScroll:GetHeight()then offset=cursorY+cursorHeight-notesScroll:GetHeight()end
        end
        notesScroll:SetVerticalScroll(math.max(0,math.min(range,offset)))
    end
    local function queueScroll()notes:SetScript("OnUpdate",updateScroll)end
    notes:SetScript("OnTextChanged",function(self)hint:SetShown(self:GetText()=="");queueScroll()end)
    notes:SetScript("OnCursorChanged",function(_,_,y,_,height)
        cursorY,cursorHeight=-y,height;queueScroll()
    end)
    notes:SetScript("OnSizeChanged",queueScroll)
    notes:SetScript("OnShow",queueScroll)
    notes:SetScript("OnEditFocusGained",function()notesBox:SetBackdropBorderColor(.2,.8,.95,1);queueScroll()end)
    notes:SetScript("OnEditFocusLost",function()notesBox:SetBackdropBorderColor(.18,.29,.36,1)end)
    notes:SetScript("OnHide",function(self)self:ClearFocus();self:SetScript("OnUpdate",nil)end)
    notes:SetScript("OnEscapePressed",function(self)self:ClearFocus()end)
    e.title:SetScript("OnTabPressed",function()scroll:SetVerticalScroll(0);notes:SetFocus()end)
    notes:SetScript("OnTabPressed",function()e.title:SetFocus()end)
    e.notesScroll=notesScroll
    e.quest=label(body,"",0,-186,810)
    e.fieldRows={}
    for i=1,16 do
        local row=CreateFrame("Frame",nil,body);row:SetSize(824,66);e.fieldRows[i]=row
        row.caption=label(row,"",0,0,824)
        row.edit=input(row,0,-25,780,28,500)
        row.check=check(row,"",0,-24,function()end)
        row.choice=select(row,0,-25,780,function()
            local values={{value="",label="(Not set)"}}
            for index,v in ipairs(row.field and row.field.options or {})do values[#values+1]={value=v,label=index..". "..v}end;return values
        end,function()return row.choiceValue or "" end,function(v)row.choiceValue=v end)
    end
    e.error=label(e,"",18,-528,870);e.error:SetTextColor(1,.65,.3)
    LUI:Button(e,"Save card",18,-575,175,function()U:SaveEditor()end)
    e.archive=LUI:Button(e,"Archive card",210,-575,185,function()
        if U.editID then U:SaveEditor(not U.editArchived)end
    end)
    LUI:Button(e,"Cancel",705,-575,185,function()e:Hide()end)
end
function U:EditCard(cardID,column)
    if self.editor:IsShown()then return end
    if LUI.openMenu then LUI.openMenu:Hide()end
    local b=K:Board();local card
    if cardID then for _,c in ipairs(b.cards)do if c.id==cardID then card=c;break end end;if not card then return end end
    self.editID=cardID;self.editArchived=card and card.archived;self.editBaseColumn=card and card.column
    self.editBoardID=b.id;self.editCharacter=K:CharacterKey()
    self.draft={column=card and card.column or column or b.columns[1].id,kind=card and card.kind or "Note",priority=card and card.priority or "Normal"}
    local e=self.editor;e.title:SetText(card and card.title or "");e.notes:SetText(card and card.notes or "");e.notes:SetCursorPosition(0)
    e.column:UpdateLabel();e.kind:UpdateLabel();e.priority:UpdateLabel();e.error:SetText("")
    e.archive:SetShown(card~=nil);e.archive:SetText(self.editArchived and "Restore card" or "Archive card")
    local index=0
    for _,field in ipairs(b.fields)do if not field.hidden then
        index=index+1;local row=e.fieldRows[index];row.field=field;row:Show();row.caption:SetText(display(field.name).." · "..field.type)
        local v=card and card.values[field.id]
        row.edit.panel:SetShown(field.type=="Text" or field.type=="Number");row.edit:SetText(v~=nil and tostring(v)or "")
        row.check:SetShown(field.type=="Checkbox");row.check:SetChecked(v==true)
        row.choice:SetShown(field.type=="Choice");row.choiceValue=v or "";row.choice:UpdateLabel()
    end end
    for i=index+1,16 do e.fieldRows[i]:Hide();e.fieldRows[i].field=nil end
    self:RefreshEditorQuest();e.scroll:SetVerticalScroll(0);e.notesScroll:SetVerticalScroll(0)
    e:Show();e.title:SetFocus()
end
function U:RefreshEditorQuest()
    local e=self.editor;local card
    local b=K:Board();if not b or self.editBoardID~=b.id or self.editCharacter~=K:CharacterKey()then return end
    for _,c in ipairs(b.cards)do if c.id==self.editID then card=c;break end end
    if card and card.column~=self.editBaseColumn then
        if self.draft.column==self.editBaseColumn then self.draft.column=card.column;e.column:UpdateLabel()end
        self.editBaseColumn=card.column
    end
    local quest=card and card.questID and ("Quest #"..card.questID.." · "..(card.questStatus or "").."\n"..(card.questProgress or ""))or ""
    e.quest:SetText(display(quest));e.quest:SetShown(quest~="")
    local y=quest~="" and (205+math.max(40,e.quest:GetStringHeight()))or 186
    for _,row in ipairs(e.fieldRows)do if row.field then row:ClearAllPoints();row:SetPoint("TOPLEFT",0,-y);y=y+70 end end
    e.body:SetHeight(math.max(332,y+12))
end
function U:SaveEditor(archived)
    local b=K:Board()
    if not b or self.editBoardID~=b.id or self.editCharacter~=K:CharacterKey()then self.editor.error:SetText("This draft belongs to another board or character. Cancel it before continuing.");return end
    local e=self.editor;local draft={title=e.title:GetText(),notes=e.notes:GetText(),column=self.draft.column,kind=self.draft.kind,priority=self.draft.priority,values={}}
    for _,row in ipairs(e.fieldRows)do if row.field then
        local field=row.field;local value
        if field.type=="Checkbox" then value=not not row.check:GetChecked()
        elseif field.type=="Choice" then value=row.choiceValue
        else value=row.edit:GetText()end
        draft.values[field.id]=value
    end end
    local card,err=K:SaveCard(self.editID,draft)
    if not card then e.error:SetText(display(err));return end
    if archived~=nil then K:ArchiveCard(card.id,archived)end
    e:Hide();self:Message(archived==true and "Card archived. Find it under Archived cards." or archived==false and "Card restored." or "Card saved.");self:Refresh()
end

-- Native column displays remain visible independently of the planner window.
-- Their saved positions belong to column IDs on this character's board.
function U:RefreshDisplayControls()
    local b=K:Board();local c=K:Column(self.selectedColumn)
    if self.columnDisplay then self.columnDisplay:SetChecked(c and c.showOnUI==true);self.columnDisplay:SetEnabled(c~=nil)end
    if self.columnLock then self.columnLock:SetChecked(c and c.uiLocked==true);self.columnLock:SetEnabled(c~=nil)end
    if self.columnTransparent then self.columnTransparent:SetChecked(c and c.uiTransparent==true);self.columnTransparent:SetEnabled(c~=nil)end
    for _,control in pairs({self.hideTracker,self.hideTrackerSettings})do control:SetChecked(b and b.hideQuestTracker==true)end
    for _,status in pairs({self.trackerStatus,self.trackerSettingsStatus})do status:SetText(self.trackerMessage or "")end
    local d=self.displayDialog
    if d and d:IsShown()then
        local column=K:Column(d.columnID)
        if not column or not b or d.boardID~=b.id or d.character~=K:CharacterKey()then d:Hide();return end
        d.placement:UpdateLabel();d.compact:SetChecked(column.uiCompact==true);d.transparent:SetChecked(column.uiTransparent==true)
        d.enabled:SetChecked(column.showOnUI==true)
        for key,control in pairs(d.contexts)do control:SetChecked(type(column.uiVisibility)~="table"or column.uiVisibility[key]~=false)end
        local context=K:DisplayContext();local name=context
        for _,entry in ipairs(K.displayContexts)do if entry[1]==context then name=entry[2];break end end
        local message="Now: "..name..". "..(K:ColumnVisible(column,context)and "This list is enabled here."or "This list is hidden here.")
        if column.uiPlacement=="tracker"or column.uiPlacement=="both"then
            message=message.."\n"..(self.nativeMessage or "Tracker sections share WoW’s tracker width and available space.")
        end
        d.status:SetText(message)
    end
end
function U:OpenDisplaySettings(columnID)
    local c=K:Column(columnID);if not c then return end
    if not self.frame then self:Open()end
    if not self.frame then return end
    if not self.displayDialog then
        local shield=CreateFrame("Frame",nil,self.frame);shield:SetAllPoints(self.frame);shield:EnableMouse(true);shield:Hide()
        shield:SetFrameLevel(self.frame:GetFrameLevel()+42)
        local d=box(self.frame,150,-60,740,580)
        self.displayDialog=d;d:Hide();d:EnableMouse(true);d:SetFrameLevel(shield:GetFrameLevel()+2)
        d:SetScript("OnShow",function()shield:Show()end)
        d:SetScript("OnHide",function()shield:Hide();d.width:ClearFocus();d.height:ClearFocus();if LUI.openMenu then LUI.openMenu:Hide()end end)
        d.title=label(d,"",20,-18,700)
        d.enabled=check(d,"Display on UI",20,-49,function(v)K:SetColumnDisplay(d.columnID,"showOnUI",v)end)
        label(d,"Placement",20,-94,120)
        d.placement=select(d,155,-87,320,function()return {{value="floating",label="Floating"},{value="tracker",label="WoW quest tracker"},{value="both",label="Both"}}end,
            function()local col=K:Column(d.columnID);return col and(col.uiPlacement or "floating")end,
            function(v)K:SetColumnPlacement(d.columnID,v)end)
        label(d,"Floating appearance",20,-135,680)
        d.compact=check(d,"Compact quest-tracker style",20,-159,function(v)K:SetColumnDisplay(d.columnID,"uiCompact",v)end);d.compact.caption:SetWidth(300)
        d.transparent=check(d,"Transparent background",380,-159,function(v)K:SetColumnDisplay(d.columnID,"uiTransparent",v)end);d.transparent.caption:SetWidth(300)
        label(d,"Width",20,-206,80);d.width=input(d,80,-199,75,30,4)
        label(d,"Height",170,-206,100);d.height=input(d,275,-199,75,30,4)
        d.apply=LUI:Button(d,"Apply size",370,-201,145,function()
            if not tonumber(d.width:GetText())or not tonumber(d.height:GetText())then d.error:SetText("Enter a width and height.");return end
            local ok,err=K:SetColumnSize(d.columnID,tonumber(d.width:GetText()),tonumber(d.height:GetText()))
            d.error:SetText(err or(ok and "Size saved."or "Enter a width and height."))
        end)
        d.reset=LUI:Button(d,"Reset size",535,-201,180,function()
            K:SetColumnSize(d.columnID);d.width:SetText("304");d.height:SetText("464");d.error:SetText("")
        end)
        label(d,"Width: 220–640. Height: 150 up to the screen height. Drag the bottom-right corner to resize.",20,-240,700)
        label(d,"Show this list in (unchecked locations auto-hide it)",20,-286,700)
        d.contexts={}
        for i,entry in ipairs(K.displayContexts)do
            local key=entry[1];local x=i<=4 and 20 or 380;local row=i<=4 and i or i-4
            local control=check(d,entry[2],x,-315-(row-1)*31,function(v)K:SetColumnVisibility(d.columnID,key,v)end)
            control.caption:SetWidth(300);d.contexts[key]=control
        end
        d.status=label(d,"",20,-452,700);d.error=label(d,"",20,-513,500);d.error:SetTextColor(1,.65,.3)
        LUI:Button(d,"Done",570,-535,145,function()d:Hide()end)
    end
    local d=self.displayDialog;d:ClearAllPoints();d:SetPoint("TOPLEFT",150,-60)
    d.columnID=columnID;d.boardID=K:Board().id;d.character=K:CharacterKey()
    local size=c.uiSize or {};d.width:SetText(tostring(size.width or 304));d.height:SetText(tostring(size.height or 464))
    d.title:SetText("Display settings / "..display(c.name));d.error:SetText("");d:Show();self:RefreshDisplayControls()
end
function U:ApplyQuestTracker(hidden)
    self.trackerMessage=""
    self.trackerParents=self.trackerParents or {}
    local frame=ObjectiveTrackerFrame or QuestWatchFrame
    if InCombatLockdown()then
        local pending=hidden and (not frame or frame:GetParent()~=self.trackerHolder)or(not hidden and next(self.trackerParents))
        if pending then self.trackerMessage="Tracker visibility will apply after combat." end
        return
    end
    if not hidden then
        for frame,parent in pairs(self.trackerParents)do
            -- Restore only our own parenting; leave another addon's changes alone.
            if frame:GetParent()~=self.trackerHolder or pcall(frame.SetParent,frame,parent)then self.trackerParents[frame]=nil
            else self.trackerMessage="Tracker restore is pending; it will retry after combat or a zone change." end
        end
        return
    end
    if not frame then self.trackerMessage="Waiting for WoW’s objective tracker to load.";return end
    if not self.trackerHolder then self.trackerHolder=CreateFrame("Frame",nil,UIParent);self.trackerHolder:Hide()end
    if frame:GetParent()~=self.trackerHolder then
        local parent=frame:GetParent() or UIParent
        if pcall(frame.SetParent,frame,self.trackerHolder)then self.trackerParents[frame]=parent
        else self.trackerMessage="Tracker visibility could not change yet; it will retry after combat or a zone change." end
    end
    -- A hidden parent keeps future Blizzard Show calls hidden, including combat.
    -- Do not replace native methods/scripts or unregister any quest events.
end
local function finite(v)return type(v)=="number" and v==v and v~=math.huge and v~=-math.huge end
local function displayMaxHeight()return math.max(150,math.min(4096,UIParent:GetHeight()-20))end
function U:PositionDisplay(frame,column,index)
    if frame.dragging or frame.resizing then return end
    local w,h=UIParent:GetWidth(),UIParent:GetHeight();if w<=0 or h<=0 then return end
    local p=column.uiPosition;local x,y
    if type(p)=="table" and finite(p.x) and finite(p.y)then x,y=p.x*w,p.y*h
    else x=w-324-((index-1)%3)*316;y=h-180-math.floor((index-1)/3)*60 end
    x=math.max(0,math.min(math.max(0,w-frame:GetWidth()),x))
    y=math.max(math.min(h,frame:GetHeight()),math.min(h,y))
    frame:ClearAllPoints();frame:SetPoint("TOPLEFT",UIParent,"BOTTOMLEFT",x,y)
    -- Keep the initial placement stable when columns are reordered or hidden.
    if type(p)~="table" or not finite(p.x) or not finite(p.y)then column.uiPosition={x=x/w,y=y/h}end
end
function U:StopDisplayDrag(frame,save)
    if not frame.dragging then return end
    frame:StopMovingOrSizing();frame.dragging=false
    local b=K:Board()
    if save and frame.character==K:CharacterKey()and b and frame.boardID==b.id then
        local w,h=UIParent:GetWidth(),UIParent:GetHeight();local x,y=frame:GetLeft(),frame:GetTop()
        if w>0 and h>0 and finite(x) and finite(y)then K:SetColumnPosition(frame.columnID,x/w,y/h)end
    end
    self:UpdateDisplayChrome(frame)
end
function U:StopDisplayResize(frame,save)
    if not frame.resizing then return end
    frame:StopMovingOrSizing();frame.resizing=false
    local b=K:Board()
    if save and b and frame.boardID==b.id and frame.character==K:CharacterKey()then
        K:SetColumnSize(frame.columnID,math.max(220,math.min(640,frame:GetWidth())),math.max(150,math.min(displayMaxHeight(),frame:GetHeight())))
    end
    self:UpdateDisplayChrome(frame)
end
function U:UpdateDisplayChrome(f)
    local show=not f.transparent or next(f.hoverTargets or {})~=nil or f.dragging or f.resizing
    f.lock:SetShown(show);f.minimize:SetShown(show)
    f.footer:SetShown(show and not f.collapsed)
    f.grip:SetShown(show and not f.collapsed and not f.locked)
    if f.displayTitle then
        local count=f.collapsed and (not f.transparent or show)and (" ("..f.cardCount..")")or ""
        f.title:SetText(f.displayTitle..count)
        f.title:SetWidth(math.max(40,f:GetWidth()-(show and 125 or 18)))
    end
end
function U:WatchDisplayHover(f,target)
    -- Resolve leave after child enter events so crossing a card or button does
    -- not flicker. Hidden controls have no invisible click targets. No polling.
    local function leave()
        f.hoverTargets[target]=nil
        if f.hoverPending then return end
        f.hoverPending=true
        C_Timer.After(0,function()f.hoverPending=false;U:UpdateDisplayChrome(f)end)
    end
    target:HookScript("OnEnter",function()f.hoverTargets[target]=true;U:UpdateDisplayChrome(f)end)
    target:HookScript("OnLeave",leave);target:HookScript("OnHide",leave)
end
function U:BuildDisplay(columnID)
    local f=box(UIParent,0,0,304,440);f:Hide();f.columnID=columnID;f.rows={};f.offset=0;f.hoverTargets={}
    f:SetFrameStrata("MEDIUM");f:SetClampedToScreen(true);f:SetMovable(true);f:SetResizable(true);f:EnableMouse(true);f:EnableMouseWheel(true)
    if f.SetResizeBounds then f:SetResizeBounds(220,150,640,displayMaxHeight())else f:SetMinResize(220,150);f:SetMaxResize(640,displayMaxHeight())end
    f.header=CreateFrame("Frame",nil,f);f.header:SetPoint("TOPLEFT");f.header:SetSize(304,29);f.header:EnableMouse(true);f.header:RegisterForDrag("LeftButton")
    f.title=label(f.header,"",9,-7,195);f.title:SetMaxLines(1)
    f.lock=check(f.header,"Lock",216,-5,function(v)K:SetColumnDisplay(f.columnID,"uiLocked",v)end);f.lock.caption:SetWidth(46)
    f.minimize=LUI:Button(f.header,"−",0,-2,26,function()
        local c=K:Column(f.columnID);if c then K:SetColumnDisplay(f.columnID,"uiCollapsed",not c.uiCollapsed)end
    end)
    LUI:Explain(f.minimize,"Minimize / expand","Keep just the column name and card count on screen. Click again to restore the list.")
    LUI:Explain(f.lock,"Lock position","Lock this column in place. Unlock to drag its header. Cards remain clickable. Hide the column using Display on UI on the planner board.")
    f.header:SetScript("OnDragStart",function()
        local c=K:Column(f.columnID)
        if c and c.uiLocked~=true and not f.resizing then f.dragging=true;f:StartMoving()end
    end)
    f.header:SetScript("OnDragStop",function()U:StopDisplayDrag(f,true)end)
    f:SetScript("OnHide",function()f.hoverTargets={};U:StopDisplayResize(f,false);U:StopDisplayDrag(f,false)end)
    f:SetScript("OnShow",function()U:UpdateDisplayChrome(f)end)
    local function scroll(delta)f.offset=math.max(0,f.offset+delta);U:RefreshDisplays()end
    f:SetScript("OnMouseWheel",function(_,delta)scroll(-delta)end)
    for i=1,20 do
        local row=CreateFrame("Button",nil,f,"BackdropTemplate");f.rows[i]=row
        row:SetPoint("TOPLEFT",7,-32-(i-1)*78);row:SetSize(290,72);row:SetBackdrop(backdrop)
        row:SetBackdropColor(.065,.10,.14,.95);row:SetBackdropBorderColor(.18,.29,.36,1)
        -- A texture cannot wrap, truncate into an ellipsis, or depend on the
        -- selected font having a bullet glyph.
        row.bulletShadow=row:CreateTexture(nil,"ARTWORK");row.bulletShadow:SetSize(10,10)
        row.bulletShadow:SetTexture("Interface\\AddOns\\LamdaFrames\\Libs\\DandersUI\\Media\\Icons\\dot.tga")
        row.bulletShadow:SetVertexColor(0,0,0,1);row.bulletShadow:SetPoint("TOPLEFT",3,-7);row.bulletShadow:Hide()
        row.bullet=row:CreateTexture(nil,"OVERLAY");row.bullet:SetSize(8,8)
        row.bullet:SetTexture("Interface\\AddOns\\LamdaFrames\\Libs\\DandersUI\\Media\\Icons\\dot.tga")
        row.bullet:SetVertexColor(1,.82,.25,1);row.bullet:SetPoint("TOPLEFT",4,-8);row.bullet:Hide()
        row.title=label(row,"",7,-5,276);row.title:SetMaxLines(2)
        row.noteScroll=CreateFrame("ScrollFrame",nil,row);row.noteScroll:SetPoint("TOPLEFT",7,-39);row.noteScroll:SetSize(276,30)
        row.noteBody=CreateFrame("Frame",nil,row.noteScroll);row.noteBody:SetSize(276,30);row.noteScroll:SetScrollChild(row.noteBody)
        row.noteScroll:EnableMouse(false);row.noteScroll:EnableMouseWheel(false);row.noteBody:EnableMouse(false)
        row.detail=label(row.noteBody,"",0,0,276);row.detail:SetMaxLines(0);row.detail:SetTextColor(.6,.82,.87)
        row:EnableMouseWheel(true);row:SetScript("OnMouseWheel",function(_,delta)
            if row.noteOverflow and row.noteOverflow>0 then
                local old=row.noteScroll:GetVerticalScroll()
                local value=math.max(0,math.min(row.noteOverflow,old-delta*36))
                if value~=old then row.noteScroll:SetVerticalScroll(value);return end
            end
            scroll(-delta)
        end)
        row:SetScript("OnClick",function()
            if not row.card then return end
            U:OpenDisplayCard(row.card.id)
        end)
        row:SetScript("OnEnter",function()
            if not row.card or not GameTooltip then return end
            local c=row.card;GameTooltip:SetOwner(row,"ANCHOR_LEFT");GameTooltip:SetText(display(c.title))
            GameTooltip:AddLine(display(c.priority.." · "..c.kind),.55,.85,.91)
            if c.questID then GameTooltip:AddLine(display((c.questStatus or "Quest").."\n"..(c.questProgress or "")),.85,.9,.95,true)end
            if c.notes~="" then GameTooltip:AddLine(displayNotes(c.notes,600),.9,.92,.95,true)end
            GameTooltip:AddLine(row.noteOverflow and row.noteOverflow>0 and "Scroll to read the rest of this note. Click to edit."or "Click to edit. Scroll for more cards.",.55,.85,.91);GameTooltip:Show()
        end)
        local function hideTip()if GameTooltip then GameTooltip:Hide()end end
        row:SetScript("OnLeave",hideTip);row:SetScript("OnHide",hideTip)
    end
    f.empty=label(f,"No active cards in this column.",12,-39,276)
    f.footer=CreateFrame("Frame",nil,f);f.footer:SetPoint("BOTTOMLEFT",7,7);f.footer:SetSize(290,26)
    f.footer:EnableMouse(true)
    f.previous=LUI:Button(f.footer,"<",0,0,28,function()scroll(-(f.previousCount or f.capacity or 5))end)
    f.next=LUI:Button(f.footer,">",262,0,28,function()scroll(f.capacity or 5)end)
    f.count=label(f.footer,"",37,-5,132);f.count:SetMaxLines(1)
    f.planner=LUI:Button(f.footer,"Planner",177,0,78,function()U:Open()end)
    f.grip=CreateFrame("Button",nil,f);f.grip:SetSize(16,16);f.grip:SetPoint("BOTTOMRIGHT",0,0)
    f.grip:SetNormalTexture("Interface\\ChatFrame\\UI-ChatIM-SizeGrabber-Up")
    f.grip:SetHighlightTexture("Interface\\ChatFrame\\UI-ChatIM-SizeGrabber-Highlight")
    f.grip:SetScript("OnMouseDown",function(_,button)
        local c=K:Column(f.columnID)
        if button=="LeftButton"and c and not c.uiLocked and not c.uiCollapsed and not f.dragging then f.resizing=true;f:StartSizing("BOTTOMRIGHT")end
    end)
    f.grip:SetScript("OnMouseUp",function()U:StopDisplayResize(f,true)end)
    f:SetScript("OnSizeChanged",function()if f.resizing then U:LayoutDisplay(f,K:Column(f.columnID))end end)
    LUI:Explain(f.grip,"Resize list","Drag to change the width and height. The list keeps the size you choose. Larger lists show more notes and cards; scroll over a very long note to read the rest.")
    for _,target in ipairs({f,f.header,f.footer,f.lock,f.minimize,f.previous,f.next,f.planner,f.grip})do self:WatchDisplayHover(f,target)end
    for _,row in ipairs(f.rows)do self:WatchDisplayHover(f,row)end
    return f
end
function U:OpenDisplayCard(cardID)
    self:Open()
    if self.editor and self.editor:IsShown()then self:Message("Save or cancel the open card before editing another.")
    elseif self.editor then self:EditCard(cardID)end
end
function U:StyleDisplay(frame,transparent)
    if frame.transparent==transparent then return end
    frame.transparent=transparent
    -- Change backdrop alpha, never frame alpha: text and mouse targets stay
    -- readable and usable even when the entire column background is clear.
    frame:SetBackdropColor(.035,.065,.09,transparent and 0 or .98)
    frame:SetBackdropBorderColor(.18,.29,.36,transparent and 0 or 1)
    local function shadow(label)label:SetShadowColor(0,0,0,1);label:SetShadowOffset(1,-1)end
    shadow(frame.title);shadow(frame.empty);shadow(frame.count);shadow(frame.lock.caption)
    for _,row in ipairs(frame.rows)do
        row:SetBackdropColor(.065,.10,.14,transparent and 0 or .95)
        row:SetBackdropBorderColor(.18,.29,.36,transparent and 0 or 1)
        shadow(row.title);shadow(row.detail)
    end
    for _,button in ipairs({frame.previous,frame.next,frame.planner,frame.minimize})do
        button:SetBackdropColor(.10,.16,.22,transparent and 0 or 1)
        button:SetBackdropBorderColor(.28,.38,.47,transparent and 0 or 1)
        shadow(button.label)
    end
end
local function cardDetail(card,limit)
    limit=limit or 300
    if card.questID then
        return display(((card.questProgress and card.questProgress~=""and card.questProgress)or card.questStatus or "Quest"):sub(1,limit))
    end
    return card.notes~=""and displayNotes(card.notes,limit)or display(card.kind)
end
local function trackerFont(text,size)
    local font=text:GetFont()or STANDARD_TEXT_FONT
    if font then text:SetFont(font,size,"OUTLINE")end
    text:SetSpacing(2);text:SetShadowColor(0,0,0,1);text:SetShadowOffset(1,-1)
end
function U:LayoutDisplay(f,c)
    if not c then return end
    local cards=K:Cards(c.id);local compact=c.uiCompact==true;local collapsed=c.uiCollapsed==true;local transparent=c.uiTransparent==true
    local step=compact and 42 or 78
    local size=type(c.uiSize)=="table"and c.uiSize or {}
    local width=finite(size.width)and math.max(220,math.min(640,size.width))or 304
    local height=finite(size.height)and math.max(150,math.min(4096,size.height))or 464
    width=math.min(width,UIParent:GetWidth());height=math.min(height,displayMaxHeight())
    if f.SetResizeBounds then f:SetResizeBounds(220,150,640,displayMaxHeight())else f:SetMaxResize(640,displayMaxHeight())end
    if f.resizing then width=f:GetWidth();height=f:GetHeight()end
    f.header:SetWidth(width);f.title:SetWidth(math.max(40,width-125))
    f.lock:ClearAllPoints();f.lock:SetPoint("TOPRIGHT",-88,-5)
    f.minimize:ClearAllPoints();f.minimize:SetPoint("TOPRIGHT",-4,-2);f.minimize:SetText(collapsed and "+"or "−")
    f.lock:SetChecked(c.uiLocked==true)
    f.displayTitle=display(c.name);f.cardCount=#cards;f.collapsed=collapsed;f.locked=c.uiLocked==true
    f.title:SetFontObject(compact and "GameFontNormal"or "GameFontHighlight")
    f.title:SetTextColor((compact or transparent)and 1 or .9,(compact or transparent)and .82 or .95,(compact or transparent)and .25 or 1)
    if transparent then trackerFont(f.title,16)end
    local function measureRow(row,card)
        row.bullet:SetShown(transparent);row.bulletShadow:SetShown(transparent)
        row.title:ClearAllPoints();row.title:SetPoint("TOPLEFT",transparent and 19 or 7,transparent and -4 or -5)
        row.title:SetWidth(width-(transparent and 40 or 28));row.title:SetMaxLines(compact and 1 or 2)
        row.title:SetFontObject(compact and "GameFontNormalSmall"or "GameFontHighlight")
        row.title:SetTextColor(1,transparent and .9 or(compact and .82 or 1),transparent and .62 or(compact and .25 or 1))
        row.detail:SetWidth(width-(transparent and 52 or 28));row.detail:SetMaxLines(0)
        row.detail:SetFontObject(compact and "GameFontHighlightSmall"or "GameFontHighlight")
        row.detail:SetTextColor(transparent and .94 or .6,transparent and .94 or .82,transparent and .94 or .87)
        row.title:SetSpacing(3);row.detail:SetSpacing(3)
        if transparent then trackerFont(row.title,14);trackerFont(row.detail,13)end
        row.title:SetText(card and display(card.title)or "")
        row.detail:SetText(card and cardDetail(card,8000)or "")
        -- Measure after setting the font, width, line limit, and text. A short
        -- title no longer reserves a second line above its description.
        local lines=compact and 1 or 2
        local titleHeight=math.ceil(math.max(14,math.min(row.title:GetStringHeight(),lines*16-2)))
        local detailHeight=math.ceil(math.max(13,row.detail:GetStringHeight()))
        local detailTop=transparent and (4+titleHeight+3)or(compact and 21 or 39)
        local gap=transparent and (compact and 4 or 8)or 6
        local rowHeight=math.max(transparent and 0 or step-6,detailTop+detailHeight+3)
        rowHeight=math.min(rowHeight,math.max(detailTop+1,height-74-gap))
        return rowHeight+gap,rowHeight,detailHeight,detailTop
    end
    local available=height-74
    local function countBefore(last)
        -- Clamp against a measured last page, so scrolling/resizing at the end
        -- keeps it filled. Only the bounded row pool needs measuring.
        local used=0;local fitted=0
        for i=1,math.min(last,#f.rows)do
            local rowStep=measureRow(f.rows[i],cards[last-i+1])
            if fitted>0 and used+rowStep>available then break end
            used=used+rowStep;fitted=fitted+1
        end
        return math.max(1,fitted)
    end
    local count=countBefore(#cards)
    f.offset=math.max(0,math.min(f.offset,math.max(0,#cards-count)))
    f.previousCount=countBefore(f.offset)
    local used=0;local shown=0;local full=false
    for i,row in ipairs(f.rows)do
        local card=not collapsed and not full and cards[f.offset+i]or nil
        local rowStep,rowHeight,detailHeight,detailTop=measureRow(row,card)
        if card and shown>0 and used+rowStep>available then card=nil;full=true end
        row.card=card;row:SetShown(card~=nil)
        row:ClearAllPoints();row:SetPoint("TOPLEFT",7,-32-used);row:SetSize(width-14,rowHeight)
        local noteWidth=width-(transparent and 52 or 28);local noteHeight=math.max(1,rowHeight-detailTop-3)
        local noteOffset=row.noteCardID==(card and card.id)and row.noteWidth==noteWidth and row.noteScroll:GetVerticalScroll()or 0
        row.noteCardID=card and card.id;row.noteWidth=noteWidth
        row.noteScroll:ClearAllPoints();row.noteScroll:SetPoint("TOPLEFT",transparent and 31 or 7,-detailTop)
        row.noteScroll:SetSize(noteWidth,noteHeight);row.noteBody:SetSize(noteWidth,detailHeight)
        row.noteScroll:UpdateScrollChildRect()
        row.noteOverflow=math.max(0,detailHeight-noteHeight)
        row.noteScroll:SetVerticalScroll(math.min(noteOffset,row.noteOverflow))
        if card then shown=shown+1;used=used+rowStep end
    end
    count=math.max(1,shown)
    f.capacity=count
    if not f.resizing then f:SetSize(width,collapsed and 30 or(finite(size.height)and height or math.min(height,74+(shown>0 and used or step))))end
    f.empty:SetWidth(width-24);f.empty:SetShown(not collapsed and #cards==0)
    f.footer:SetWidth(width-24)
    f.next:ClearAllPoints();f.next:SetPoint("TOPRIGHT",0,0)
    f.planner:ClearAllPoints();f.planner:SetPoint("TOPRIGHT",-35,0)
    f.count:SetWidth(math.max(30,width-182))
    f.count:SetText(#cards==0 and "0 cards"or ((f.offset+1).."–"..math.min(f.offset+count,#cards).." of "..#cards))
    f.previous:SetEnabled(f.offset>0);f.next:SetEnabled(f.offset+count<#cards)
    self:UpdateDisplayChrome(f)
end

-- Native sections use Blizzard's own block/layout pools. Reuse at most twelve
-- slots across boards; inactive slots lay out nothing and release their blocks.
-- Never replace a Blizzard module or alter its quests/events/layout methods.
function U:NativeTrackerAvailable()
    return ObjectiveTrackerManager and ObjectiveTrackerFrame and ObjectiveTrackerModuleMixin
        and type(ObjectiveTrackerManager.SetModuleContainer)=="function"
        and ObjectiveTrackerManager.containers and ObjectiveTrackerManager.containers[ObjectiveTrackerFrame]
end
function U:NativeColumn(module)
    local b=K:Board()
    if not module.active or not b or b.id~=module.boardID or K:CharacterKey()~=module.character
        or not LamdaEngineDB or LamdaEngineDB.kbbEnabled==false then return end
    local c=K:Column(module.columnID)
    if c and K:ColumnVisible(c)and(c.uiPlacement=="tracker"or c.uiPlacement=="both")then return c end
end
function U:BuildNativeSection(index)
    local ok,module=pcall(CreateFrame,"Frame",nil,UIParent,"ObjectiveTrackerModuleTemplate")
    if not ok then return end
    module.uiOrder=1000+index;module.active=false
    module.LayoutContents=function(m)
        local c=U:NativeColumn(m);if not c then return end
        local cards=K:Cards(c.id);m:SetHeader("KBB: "..display(c.name).." ("..#cards..")")
        if #cards==0 then
            local block=m:GetBlock("empty");block.kbbCardID=nil;block.kbbBoardID=m.boardID;block.kbbCharacter=m.character
            block:SetHeader("No active cards");m:LayoutBlock(block);return
        end
        for _,card in ipairs(cards)do
            local block=m:GetBlock(card.id);block.kbbCardID=card.id;block.kbbBoardID=m.boardID;block.kbbCharacter=m.character
            block:SetHeader(display(card.title));block:AddObjective("detail",cardDetail(card))
            if not m:LayoutBlock(block)then break end
        end
    end
    module.OnBlockHeaderClick=function(m,block,button)
        if button=="LeftButton"and U:NativeColumn(m)and block.kbbBoardID==m.boardID and block.kbbCharacter==m.character then
            if block.kbbCardID then U:OpenDisplayCard(block.kbbCardID)else U:Open()end
        end
    end
    module.OnBlockHeaderEnter=function(m,block)
        if not U:NativeColumn(m)or block.kbbBoardID~=m.boardID or block.kbbCharacter~=m.character or not GameTooltip then return end
        local card
        for _,candidate in ipairs(K:Board().cards)do if candidate.id==block.kbbCardID then card=candidate;break end end
        GameTooltip:SetOwner(block,"ANCHOR_LEFT");GameTooltip:SetText(card and display(card.title)or "LamdaKBB")
        if card and card.notes~=""then GameTooltip:AddLine(displayNotes(card.notes,600),.9,.92,.95,true)end
        GameTooltip:AddLine("Click to open the planner. Expand the tracker or untrack objectives if a list is cut off.",.55,.85,.91,true);GameTooltip:Show()
    end
    module.OnBlockHeaderLeave=function()if GameTooltip then GameTooltip:Hide()end end
    local setCollapsed=module.SetCollapsed
    module.SetCollapsed=function(m,collapsed)
        setCollapsed(m,collapsed)
        if not m.syncing then
            local c=U:NativeColumn(m)
            if c then c.uiCollapsed=collapsed==true;C_Timer.After(0,function()U:RefreshDisplays()end)end
        end
    end
    ObjectiveTrackerManager:SetModuleContainer(module,ObjectiveTrackerFrame)
    return module
end
function U:RefreshNativeSections(b,enabled)
    self.nativeSections=self.nativeSections or {};local active={};local available=self:NativeTrackerAvailable()
    self.nativeMessage=nil
    if not available then self.nativeMessage="WoW tracker sections are unavailable; this list uses a floating view."
    elseif b and b.hideQuestTracker then self.nativeMessage="WoW’s tracker is hidden by this board. A floating view is used until you show it on Quests."
    elseif InCombatLockdown()then self.nativeMessage="New tracker sections attach after combat. Floating lists remain available meanwhile."end
    if b and enabled and available and not b.hideQuestTracker then
        for index,c in ipairs(b.columns)do
            if K:ColumnVisible(c)and(c.uiPlacement=="tracker"or c.uiPlacement=="both")then
                local m=self.nativeSections[index]
                if not m and not InCombatLockdown()then m=self:BuildNativeSection(index);self.nativeSections[index]=m end
                if m then
                    m.active=true;m.columnID=c.id;m.boardID=b.id;m.character=K:CharacterKey()
                    m.syncing=true;m:SetCollapsed(c.uiCollapsed==true);m.syncing=false;m:MarkDirty();active[index]=true
                else self.nativeMessage="Waiting to attach this tracker section; a floating view is available meanwhile."end
            end
        end
    end
    for index,m in pairs(self.nativeSections)do if not active[index]then m.active=false;m:MarkDirty()end end
end
function U:RefreshDisplays()
    local b=K:Board();local enabled=type(LamdaEngineDB)=="table"and LamdaEngineDB.kbbEnabled~=false
    self:ApplyQuestTracker(enabled and b and b.hideQuestTracker==true)
    self:RefreshNativeSections(b,enabled)
    self.displays=self.displays or {}
    local visible={};local character=K:CharacterKey();local displayIndex=0;local context=K:DisplayContext()
    if enabled and b then for index,c in ipairs(b.columns)do
        local native=self.nativeSections[index]
        local attached=native and native.active and not b.hideQuestTracker and self:NativeTrackerAvailable()
        if K:ColumnVisible(c,context)and(c.uiPlacement~="tracker"or not attached)then
            displayIndex=displayIndex+1
            local f=self.displays[c.id]
            if not f then f=self:BuildDisplay(c.id);self.displays[c.id]=f end
            if f.character~=character or f.boardID~=b.id then
                self:StopDisplayResize(f,false);self:StopDisplayDrag(f,false);f.character=character;f.boardID=b.id;f.offset=0
            end
            if c.uiLocked==true or c.uiCollapsed==true then self:StopDisplayResize(f,false)end
            if c.uiLocked==true then self:StopDisplayDrag(f,true)end
            self:StyleDisplay(f,c.uiTransparent==true);self:LayoutDisplay(f,c)
            self:PositionDisplay(f,c,displayIndex);f:Show();visible[c.id]=true
        end
    end end
    for id,f in pairs(self.displays)do if not visible[id]then self:StopDisplayResize(f,false);self:StopDisplayDrag(f,false);f:Hide()end end
    self:RefreshDisplayControls()
end
