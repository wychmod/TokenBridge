export namespace main {
	
	export class DesktopCheckItem {
	    key: string;
	    title: string;
	    description: string;
	    status: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new DesktopCheckItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	    }
	}
	export class DesktopConfigSummary {
	    host: string;
	    port: number;
	    adminPath: string;
	    theme: string;
	    bundleMode: string;
	    updateChannel: string;
	
	    static createFrom(source: any = {}) {
	        return new DesktopConfigSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.adminPath = source["adminPath"];
	        this.theme = source["theme"];
	        this.bundleMode = source["bundleMode"];
	        this.updateChannel = source["updateChannel"];
	    }
	}
	export class DesktopRuntimeSummary {
	    providers: number;
	    keys: number;
	    rules: number;
	    health: string;
	
	    static createFrom(source: any = {}) {
	        return new DesktopRuntimeSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providers = source["providers"];
	        this.keys = source["keys"];
	        this.rules = source["rules"];
	        this.health = source["health"];
	    }
	}
	export class DesktopSelfCheck {
	    health: string;
	    checks: DesktopCheckItem[];
	    warnings: string[];
	    completedAt: string;
	    serverReachable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DesktopSelfCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.health = source["health"];
	        this.checks = this.convertValues(source["checks"], DesktopCheckItem);
	        this.warnings = source["warnings"];
	        this.completedAt = source["completedAt"];
	        this.serverReachable = source["serverReachable"];
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
	export class DesktopWindowState {
	    width: number;
	    height: number;
	    x: number;
	    y: number;
	    maximised: boolean;
	    lastRoute: string;
	    hiddenToTray: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DesktopWindowState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.maximised = source["maximised"];
	        this.lastRoute = source["lastRoute"];
	        this.hiddenToTray = source["hiddenToTray"];
	    }
	}
	export class DesktopStatus {
	    version: string;
	    platform: string;
	    serverAddr: string;
	    adminUrl: string;
	    windowTitle: string;
	    desktopMode: boolean;
	    notifications: boolean;
	    customChrome: boolean;
	    trayEnabled: boolean;
	    hideToTrayEnabled: boolean;
	    stateRestore: boolean;
	    windowState: DesktopWindowState;
	    runtime: DesktopRuntimeSummary;
	    configSummary: DesktopConfigSummary;
	
	    static createFrom(source: any = {}) {
	        return new DesktopStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.platform = source["platform"];
	        this.serverAddr = source["serverAddr"];
	        this.adminUrl = source["adminUrl"];
	        this.windowTitle = source["windowTitle"];
	        this.desktopMode = source["desktopMode"];
	        this.notifications = source["notifications"];
	        this.customChrome = source["customChrome"];
	        this.trayEnabled = source["trayEnabled"];
	        this.hideToTrayEnabled = source["hideToTrayEnabled"];
	        this.stateRestore = source["stateRestore"];
	        this.windowState = this.convertValues(source["windowState"], DesktopWindowState);
	        this.runtime = this.convertValues(source["runtime"], DesktopRuntimeSummary);
	        this.configSummary = this.convertValues(source["configSummary"], DesktopConfigSummary);
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

