'use strict';

var emit = function(){

};

var func = function(doc){

	doc.names.forEach(
		function(name,index){
			emit([doc.id,index],name);
		} 

	);

	doc.externalIds.forEach(
		function(externalId){

			emit([doc.id,externalId.providerId],externalId);
		}
	);

	doc.level1.forEach(
		function(level1){
			level1.level2.forEach(
				function(level2){
					emit([doc.id,level1.level1param,level2.level2param],level2);
				}
			);
		}
	);

};

func(1);

var func2 = function(doc){
	doc.languageproficiencies.forEach( 
		function(languageproficiency){  
			emit([doc.id,languageproficiency.pos],languageproficiency);  
		} );
	}