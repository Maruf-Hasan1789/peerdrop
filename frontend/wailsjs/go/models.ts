export namespace app {
	
	export class Settings {
	    user_name: string;
	    download_path: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user_name = source["user_name"];
	        this.download_path = source["download_path"];
	    }
	}
	export class transferHistory {
	    receiver: string;
	    fileName: string;
	    status: string;
	    timeStamp: number;
	
	    static createFrom(source: any = {}) {
	        return new transferHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.receiver = source["receiver"];
	        this.fileName = source["fileName"];
	        this.status = source["status"];
	        this.timeStamp = source["timeStamp"];
	    }
	}

}

export namespace discovery {
	
	export class Peer {
	    ID: string;
	    Name: string;
	    Addresses: number[][];
	    Port: number;
	    Version: string;
	
	    static createFrom(source: any = {}) {
	        return new Peer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Addresses = source["Addresses"];
	        this.Port = source["Port"];
	        this.Version = source["Version"];
	    }
	}
	export class PeerDTO {
	    id: string;
	    name: string;
	    addresses: string[];
	    port: number;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new PeerDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.addresses = source["addresses"];
	        this.port = source["port"];
	        this.version = source["version"];
	    }
	}

}

export namespace session {
	
	export class PeerSession {
	
	
	    static createFrom(source: any = {}) {
	        return new PeerSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

