<?php
/**
 * Config_Lite_Exception_UnexpectedValue (Config/Lite/Exception/UnexpectedValue.php)
 *
 * PHP version 5
 *
 * @file      Config/Lite/Exception/UnexpectedValue.php
 * @category  Configuration
 * @package   Config_Lite
 * @author    Patrick C. Engel <pce@php.net>
 * @copyright 2010-2011 <pce@php.net>
 * @license   http://www.gnu.org/copyleft/lesser.html  LGPL License 2.1
 * @version   SVN: $Id$
 * @link      https://github.com/pce/config_lite
 */


/**
 * Config_Lite_Exception_UnexpectedValue
 *
 * implements Config_Lite_Exception
 *
 * @category  Configuration
 * @package   Config_Lite
 * @author    Patrick C. Engel <pce@php.net>
 * @copyright 2010-2011 <pce@php.net>
 * @license   http://www.gnu.org/copyleft/lesser.html  LGPL License 2.1
 * @version   Release: 0.2.5
 * @link      https://github.com/pce/config_lite
 */

class Config_Lite_Exception_UnexpectedValue 
              extends UnexpectedValueException 
              implements Config_Lite_Exception
{
}
