export namespace config {
	
	export class ServerEndpoint {
	    name: string;
	    description: string;
	    address: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerEndpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.address = source["address"];
	        this.port = source["port"];
	    }
	}
	export class Config {
	    installDir: string;
	    cdnBaseUrl: string;
	    gameExe: string;
	    servers: ServerEndpoint[];
	    activeServer: number;
	    runtimeMode: string;
	    protonPath: string;
	    protonVersion: string;
	    prefixPath: string;
	    enableDxvk: boolean;
	    enableGameMode: boolean;
	    enableMangoHud: boolean;
	    launchArgs: string;
	    apiBaseUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installDir = source["installDir"];
	        this.cdnBaseUrl = source["cdnBaseUrl"];
	        this.gameExe = source["gameExe"];
	        this.servers = this.convertValues(source["servers"], ServerEndpoint);
	        this.activeServer = source["activeServer"];
	        this.runtimeMode = source["runtimeMode"];
	        this.protonPath = source["protonPath"];
	        this.protonVersion = source["protonVersion"];
	        this.prefixPath = source["prefixPath"];
	        this.enableDxvk = source["enableDxvk"];
	        this.enableGameMode = source["enableGameMode"];
	        this.enableMangoHud = source["enableMangoHud"];
	        this.launchArgs = source["launchArgs"];
	        this.apiBaseUrl = source["apiBaseUrl"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace launcher {
	
	export class GameStatus {
	    running: boolean;
	    pid: number;
	    exitCode: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new GameStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.pid = source["pid"];
	        this.exitCode = source["exitCode"];
	        this.error = source["error"];
	    }
	}

}

export namespace main {
	
	export class PlatformInfo {
	    os: string;
	    arch: string;
	
	    static createFrom(source: any = {}) {
	        return new PlatformInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	    }
	}

}

export namespace neocronapi {
	
	export class Application {
	    id: number;
	    name: string;
	    description: string;
	    key: string;
	    executable: string;
	    endpoint: string;
	    updateUri: string;
	    server: string;
	    type: string;
	    newsFeedUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Application(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.key = source["key"];
	        this.executable = source["executable"];
	        this.endpoint = source["endpoint"];
	        this.updateUri = source["updateUri"];
	        this.server = source["server"];
	        this.type = source["type"];
	        this.newsFeedUrl = source["newsFeedUrl"];
	    }
	}
	export class Endpoint {
	    name: string;
	    description: string;
	    endpoint: string;
	    serverFlags: number;
	
	    static createFrom(source: any = {}) {
	        return new Endpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.endpoint = source["endpoint"];
	        this.serverFlags = source["serverFlags"];
	    }
	}
	export class SessionDetails {
	    requestSucceeded: boolean;
	    exceptionMessage?: string;
	    token: string;
	    name: string;
	    isLoggedIn: boolean;
	    isGameMaster: boolean;
	    backendVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestSucceeded = source["requestSucceeded"];
	        this.exceptionMessage = source["exceptionMessage"];
	        this.token = source["token"];
	        this.name = source["name"];
	        this.isLoggedIn = source["isLoggedIn"];
	        this.isGameMaster = source["isGameMaster"];
	        this.backendVersion = source["backendVersion"];
	    }
	}

}

export namespace proton {
	
	export class Build {
	    name: string;
	    path: string;
	    source: string;
	    version: string;
	    valid: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Build(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.source = source["source"];
	        this.version = source["version"];
	        this.valid = source["valid"];
	    }
	}
	export class GHAsset {
	    name: string;
	    browser_download_url: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new GHAsset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.browser_download_url = source["browser_download_url"];
	        this.size = source["size"];
	    }
	}
	export class GHRelease {
	    tag_name: string;
	    name: string;
	    assets: GHAsset[];
	
	    static createFrom(source: any = {}) {
	        return new GHRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tag_name = source["tag_name"];
	        this.name = source["name"];
	        this.assets = this.convertValues(source["assets"], GHAsset);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PrefixStatus {
	    initialized: boolean;
	    depsInstalled: boolean;
	    path: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PrefixStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.initialized = source["initialized"];
	        this.depsInstalled = source["depsInstalled"];
	        this.path = source["path"];
	        this.message = source["message"];
	    }
	}

}

export namespace updater {
	
	export class Progress {
	    totalFiles: number;
	    currentFile: number;
	    currentName: string;
	    percent: number;
	    bytesTotal: number;
	    bytesDone: number;
	    speed: number;
	    skippedFiles: number;
	    status: string;
	    errorMessage?: string;
	
	    static createFrom(source: any = {}) {
	        return new Progress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalFiles = source["totalFiles"];
	        this.currentFile = source["currentFile"];
	        this.currentName = source["currentName"];
	        this.percent = source["percent"];
	        this.bytesTotal = source["bytesTotal"];
	        this.bytesDone = source["bytesDone"];
	        this.speed = source["speed"];
	        this.skippedFiles = source["skippedFiles"];
	        this.status = source["status"];
	        this.errorMessage = source["errorMessage"];
	    }
	}
	export class UpdateCheckResult {
	    needsUpdate: boolean;
	    serverVersion: string;
	    localVersion: string;
	    isInstalled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.needsUpdate = source["needsUpdate"];
	        this.serverVersion = source["serverVersion"];
	        this.localVersion = source["localVersion"];
	        this.isInstalled = source["isInstalled"];
	    }
	}

}

