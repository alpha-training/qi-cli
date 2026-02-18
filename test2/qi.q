\d .qi

env:{$[count v:getenv x;v;y]}
HOME:env[`HOME;env[`USERPROFILE;"."]]

/ can override the below with an env variable or entry in .qi.conf
.conf.URL:env[`QI_URL;"https://raw.githubusercontent.com/alpha-training/qi/refs/heads/main/.qi.json"]
.conf.API:env[`QI_API;"https://api.github.com/repos/"]
.conf.RAW:env[`QI_REPO_RAW;"https://raw.githubusercontent.com/"]
.conf.TOKEN:env[`GITHUB_TOKEN;""]

.conf.QI_HOME:env[`QI_HOME;HOME,"/.qi"]
.conf.DATA:env[`QI_DATA;HOME,"/data"]
.conf.LOGS:env[`QI_LOGS;.conf.DATA,"/logs"]
/.conf.HUB:env[`QI_HUB;"8000"]

LOCAL:hsym`$$[WIN:.z.o like"w*";ssr[first system"cd";"\\";"/"];first system"pwd"]
.conf.STACKS:env[`QI_STACKS;1_string` sv LOCAL,`stacks]

/ can be overriden with qi.conf
.conf.AUTO_VENDOR:0

/ file system
mv:("mv";"move")WIN:.z.o like"w*"
tostr:{$[0=count x;"";0=t:type x;.z.s each x;t in -10 10h;x;string x]}
tosym:{$[0=count x;`$();0=t:type x;.z.s each x;t in -11 11h;x;`$tostr x]}
path:{$[0>type x;hsym tosym x;` sv @[raze tosym x;0;hsym]]}  / `:path/to/file
spath:1_string path@                                         / "path/to/file"
ospath:$[WIN;ssr[;"/";"\\"]spath@;spath]                       / "path/to/file (Mac/Linux) path\to\file (Windows)"
local:{path(LOCAL;x)}
qihome:{path(.conf.QI_HOME;x)} 
exists:{not()~key path x}
isfile:{p~key p:path x}
ext:{$[x like"*",y;x;`$tostr[x],y]}
dotq:ext[;".q"]
paths:{a where(last each` vs'a:(raze/){$[p~k:key p:path x;p;.z.s each` sv'p,'k where not k like".*"]}x)like tostr y}
hostport:{`$$[":"=f:first a:tostr x;a;f in .Q.n;"::",a;":",a]}

/ basic logging function
print:{[typ;msg] -1 string[.z.p]," ",typ," ",string[.z.w]," ",$[10=abs type msg;msg;-3!msg];}
info:print"info"

/ try-catch
tryx:{[func;args;catch] $[`ERR~first r:.[func;args;{(`ERR;x)}];(0b;catch;r 1);(1b;r;"")]}
try:{tryx[x;enlist y;z]}    / for monadic (1 arg) functions

/ web & json
curl:{system("curl -fsSL ",$[count tk:.conf.TOKEN;"-H \"Authorization: Bearer ",tk,"\" ";""]),x}
jcurl:.j.k raze curl@
fetch:{[url;p]
  info "fetch: ",cmd:"curl -L -s -o ",ospath[p]," ",url;
  path[p]1:0#0x;
  system cmd;
  }
  
readj:.j.k raze read0 path@
formatj:{o:x in"{[";p:o-c:(n:next x)in"}]";l:o|c|x=",";w:("\""=prev x)&(x=":")&n<>" ";"\n"vs raze x,'(w#'" "),'(l#'"\n"),'(2*l*sums p)#'" "}
readpkgs:{[p] ([]k:key a)!get a:readj[p]`packages}

/ config loading
infer:{
  if[(t:type x)in 0 98 99h;:.z.s each x];
  if[t<>10;:x];
  if[x~enlist"*";:"*"];
  if[a~inter[a:-1_x]v:.Q.n," .:-";:get x];
  if[" "in x;:.z.s each" "vs x];
  if[x[0 10]like"[1-2]D";if[not null p:"P"$x;:p]];
  $[":"=x 0;`$x;0=s:sum x="`";x;"`"<>x 0;x;`$1_$[s=1;x;"`"vs x]]}

parseconf:{[p]
  s@:where(s:read0 p)like"[A-z]*";
  s@:where 1=sum each s="=";
  s:trim @[s;where"#"in's;first"#"vs];
  if[count err:select from(a:flip`k`v!("S*";"=")0:s)where 0=count each v;
    show err;.log.fatal"Badly formed ",1_string p];
  (1#.q),a[`k]!infer each a`v}

loadconfx:{[required;p] if[not exists cp:path p;if[required;'"loadconf - ",spath[cp]," not found"];:()];.conf,:parseconf cp;}
loadconf:loadconfx 0b

/ package management
pkgs:1#.q
loadf:{[p] system"l ",spath p;}

loadpkg:{[init;p;name]
  pkgs[name]:p;
  loadconf(p;`defaults.conf);
  loadschemas name;
  if[name in`cli;:.log.info string[name]," installed"];
  if[init;
    system"d .";
    loadf(p;dotq name);
    if[name=`log;info::.log.info]]}

frompkg:{[pkg;f]
  if[null p:pkgs pkg;
    import pkg;
    p:pkgs pkg];
  loadf(p;dotq f);
  }

load1schema:{[p]
  info "load1schema ",tostr f:last` vs p;
  tab:first` vs f;
  a:("SC";",")0:p;
  sv[`;`.schemas.t,tab]set r:flip a[0]!a[1]$\:();
  sv[`;`.schemas.c,tab]set a 0;
  @[`.;tab;:;r]
  }

loadschemas:{[pkg] load1schema each paths[path(pkgs pkg;`schemas);"*.csv"];}

getopt:{$[(::)~o:opts x;"";infer o]}

checkpackages:{
  if[not exists f:local`.qi`index.json;fetch[.conf.URL;f]];
  if[not`packages in key`.qi;packages::update sha:{""}each i from readpkgs f]}

parsecmd:{[cmd]
  checkpackages[];
  if[cmd~"list";
    if[1=count .z.x;show select package:k,github_repo:`$repo from .qi.packages;exit 0]];
  getinitf:{@[get;` sv `,x,`init;::]};
  if[count select from (pk:0!.qi.packages)where k like cmd;
    import cmd;
    if[`init in key opts;getinitf[.qi.tosym cmd][]];   / TODO - ever used?
    :()];
  if[isproc::0<count a:select from pk where cmd like/:(string[k],'"*");
    import pkg:first a`k;
    .qi.import`proc;
    .proc.init .qi.tosym cmd;
    getinitf[pkg][]];
  }

requireconfs:{[c] if[count m:((),c)except key .conf;'"Missing required setting(s) in .conf: ",","sv string m]}

importx:{[init;x]
  if[not null pkgs name:first` vs sx:tosym x;:(::)];
  if[name in key`;:(::)];
  $[`log=tosym x;print["info"];info]"Loading ",string name;
  if[exists pv:local`vendor,name;
    :loadpkg[init;pv;name]];
  checkpackages[];
  if[exists lockf:local`.qi`index.lock.json;
    packages,:readpkgs lockf];
  if[not count repo:(m:packages name)`repo;'string[name]," is not a valid package"];
  if[nosha:not count sha:m`sha;
    isTag:m[`ref]like"v[0-9]*";
    obj:jcurl[.conf.API,repo,"/git/refs/",$[isTag;"tags";"heads"],"/",m`ref]`object;
    sha:obj`sha;
    if[isTag;
      if["tag"~obj`type;
        sha:jcurl[.conf.API,repo,"/git/tags/",obj`sha][`object]`sha]]];
  if[nosha;
    info"Writing new lock file";
    packages[name;`sha]:sha;
    lockf 0:formatj .j.j {enlist[`packages]!enlist(exec k from key x)!get x}select from packages where 0<count each sha];
  vfiles:();
  if[not exists dir:qihome(`cache;repo;sha);
    tree_sha:jcurl[.conf.API,repo,"/git/commits/",sha][`tree]`sha;
    treeInfo:`typ xcol`type`path#/:jcurl[.conf.API,repo,"/git/trees/",tree_sha,"?recursive=1"]`tree;
    {[name;repo;sha;file]
      url:.conf.RAW,repo,"/",sha,"/",f:file;
      isexec:0b;  / is executable
      if[name=`cli;
        if[isexec:f like"dist/*";
          if[not $[WIN;f like"*.exe";.z.o like"l*";f like"*linux*";f like"*",(3#first system"uname -m"),"*"];:()];
          f:first"-"vs 5_f]];
      if[not exists p:qihome(`cache;repo;sha;f);
        fetch[url;p];
        if[isexec&not WIN;@[system;"chmod +x ",sp;{.log.error"Failed to set +x perms on ",x,": ",y}sp:spath p]]];
      }[name;repo;sha]each vfiles:exec path from treeInfo where typ like"blob"];
  if[vend:.conf.AUTO_VENDOR;
    info"Vendoring ",string[name]," to ",spath pv;
    {[src;targ;f] path[(targ;f)]0:read0 path(src;f)}[dir;pv]each $[count vfiles;vfiles;key dir]];
  loadpkg[init;$[vend;pv;dir];name]}

import:importx 1b

opts:(1#.q),first each .Q.opt .z.x
isproc:any`name`proc in key opts 
doctor:`doctor in key opts
loadconf (.conf.QI_HOME;`qi.conf);
loadconf local`.qi`qi.conf;

.qi.importx[0b]each `$lower","vs .qi.getopt`schemas;
.qi.isproc:0b
{if[10=type x;.qi.parsecmd x]}first .z.x;