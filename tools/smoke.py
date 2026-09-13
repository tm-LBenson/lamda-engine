"""End-to-end engine test using synthetic file arrivals, not a game simulation."""
import datetime,json,os,pathlib,subprocess,sys,tempfile,time
binary=str(pathlib.Path(sys.argv[1]).resolve())
with tempfile.TemporaryDirectory() as tmp:
    root=pathlib.Path(tmp); (root/'Logs').mkdir()
    log=root/'Logs/WoWCombatLog-test.txt';log.write_text('old data\n')
    cfg=root/'settings.lua'
    def config(enabled):
        cfg.write_text('LamdaEngineDB = {\n'+ '\n'.join(f'["{k}"] = {v},' for k,v in dict(schema=1,cdEnabled=str(enabled).lower(),checkUpdates='false',notifyUpdates='false',checkDays=1,overlayX=60,overlayY=240,overlayScale=1).items())+'\n}\n')
    config(True)
    env=dict(os.environ,XDG_CONFIG_HOME=str(root/'config'))
    p=subprocess.Popen([binary,'--no-overlay','--retail',str(root),'--config',str(cfg)],env=env)
    state=root/'config/LamdaUI/state.json'
    def wait_for(predicate):
        until=time.monotonic()+6
        while time.monotonic()<until:
            try:
                data=json.loads(state.read_text())
                if predicate(data):return data
            except (FileNotFoundError,json.JSONDecodeError):pass
            if p.poll() is not None:raise AssertionError('engine exited')
            time.sleep(.1)
        raise AssertionError('state timeout')
    try:
        wait_for(lambda s:not s['rows'])
        at=datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(seconds=4)
        stamp=at.strftime('%m/%d/%Y %H:%M:%S.')+f'{at.microsecond//1000:03d}+0'
        with log.open('a') as f:f.write(stamp+'  SPELL_CAST_SUCCESS,Player-1-TEST,"Test",0x512,0x0,0000000000000000,nil,0x80000000,0x0,55342,"Mirror Image",0x40\n')
        result=wait_for(lambda s:len(s['rows'])==1)
        assert 3.5<result['rows'][0]['delay']<6
        assert 113<result['rows'][0]['ends']-time.time()<117
        config(False)
        wait_for(lambda s:not s['config']['enabled'] and not s['rows'])
        print('PASS: file arrival -> timer; original cast timestamp; saved settings -> disabled module')
    finally:
        p.send_signal(__import__('signal').SIGINT)
        p.wait(timeout=5)
