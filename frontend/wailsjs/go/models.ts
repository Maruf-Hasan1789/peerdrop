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

