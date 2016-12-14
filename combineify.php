<?php
	require dirname(__FILE__) . '/vendor/autoload.php';
	use MatthiasMullie\Minify;
	/************************************************************************
	 * CSS and Javascript Combinator 0.6
	 * Copyright 2006 by Niels Leenheer
	 * Updated 2016 - d8ahazard
	 */
	
	
	$cache 	  = true;
	$cachedir = dirname(__FILE__) . '/cache';
	if ((!file_exists($cachedir)) && $cache) {
		mkdir($cachedir, 0777, true);
	}

	$type = $_GET['type'];
	$elements = explode(',', $_GET['files']);
	
	// Determine last modification date of the files
	$lastmodified = 0;
	while (list(,$element) = each($elements)) {
		$path = realpath($element);
	
		if (($type == 'javascript' && substr($path, -3) != '.js') || 
			($type == 'css' && substr($path, -4) != '.css')) {
			header ("HTTP/1.0 403 Forbidden");
			exit;	
		}
	
		if (!file_exists($path)) {
			header ("HTTP/1.0 404 Not Found");
			exit;
		}
		
		$lastmodified = max($lastmodified, filemtime($path));
	}
	
	// Send Etag hash
	$hash = $lastmodified . '-' . md5($_GET['files']);
	header ("Etag: \"" . $hash . "\"");
	
	if (isset($_SERVER['HTTP_IF_NONE_MATCH']) && 
		stripslashes($_SERVER['HTTP_IF_NONE_MATCH']) == '"' . $hash . '"') 
	{
		// Return visit and no modifications, so do not send anything
		header ("HTTP/1.0 304 Not Modified");
		header ('Content-Length: 0');
	} 
	else 
	{
		// First time visit or files were modified
		if ($cache) 
		{
			// Determine supported compression method
			$gzip = strstr($_SERVER['HTTP_ACCEPT_ENCODING'], 'gzip');
			$deflate = strstr($_SERVER['HTTP_ACCEPT_ENCODING'], 'deflate');
	
			// Determine used compression method
			$encoding = $gzip ? 'gzip' : ($deflate ? 'deflate' : 'none');
	
			// Check for buggy versions of Internet Explorer
			if (!strstr($_SERVER['HTTP_USER_AGENT'], 'Opera') && 
				preg_match('/^Mozilla\/4\.0 \(compatible; MSIE ([0-9]\.[0-9])/i', $_SERVER['HTTP_USER_AGENT'], $matches)) {
				$version = floatval($matches[1]);
				
				if ($version < 6)
					$encoding = 'none';
					
				if ($version == 6 && !strstr($_SERVER['HTTP_USER_AGENT'], 'EV1')) 
					$encoding = 'none';
			}
			
			// Try the cache first to see if the combined files were already generated
			$cachefile = 'cache-' . $hash . '.' . $type . ($encoding != 'none' ? '.' . $encoding : '');
			
			if (file_exists($cachedir . '/' . $cachefile)) {
				if ($fp = fopen($cachedir . '/' . $cachefile, 'rb')) {

					if ($encoding != 'none') {
						header ("Content-Encoding: " . $encoding);
					}
				
					header ("Content-Type: text/" . $type);
					header ("Content-Length: " . filesize($cachedir . '/' . $cachefile));
		
					fpassthru($fp);
					fclose($fp);
					exit;
				}
			}
		}
	
		// Get contents of the files
		$contents = '';
		reset($elements);
		if ($type == 'css') {
			$minifier = new Minify\CSS();	
		}
		if ($type =='javascript') {
			$minifier = new Minify\JS();	
		}
		while (list(,$element) = each($elements)) {
			$path = realpath($element);
			$minifier->add($path);
		}
					
		$contents = $minifier->minify();
		
			
		// Send Content-Type
		header ("Content-Type: text/" . $type);
		
		if (isset($encoding) && $encoding != 'none') 
		{
			// Send compressed contents
			$contents = gzencode($contents, 9, $gzip ? FORCE_GZIP : FORCE_DEFLATE);
			header ("Content-Encoding: " . $encoding);
			header ('Content-Length: ' . strlen($contents));
			echo $contents;
		} 
		else 
		{
			// Send regular contents
			header ('Content-Length: ' . strlen($contents));
			echo $contents;
		}

		// Store cache
		if ($cache) {
			if ($fp = fopen($cachedir . '/' . $cachefile, 'wb')) {
				fwrite($fp, $contents);
				fclose($fp);
			}
		}
	}	
	
