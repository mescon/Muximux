<?php
/**
 * Config_Lite (Config/Lite.php)
 *
 * PHP version 5
 *
 * @file      Config/Lite.php
 * @category  Configuration
 * @package   Config_Lite
 * @author    Patrick C. Engel <pce@php.net>
 * @copyright 2010-2011 <pce@php.net>
 * @license   http://www.gnu.org/copyleft/lesser.html  LGPL License 2.1
 * @version   SVN: $Id$
 * @link      https://github.com/pce/config_lite
 */

require_once 'Config/Lite/Exception.php';
require_once 'Config/Lite/Exception/InvalidArgument.php';
require_once 'Config/Lite/Exception/Runtime.php';
require_once 'Config/Lite/Exception/UnexpectedValue.php';

/**
 * Config_Lite Class
 *
 * read and save ini text files.
 * Config_Lite has the native PHP function
 * `parse_ini_file' under the hood.
 * The API is inspired by Python's ConfigParser.
 * A "Config_Lite" file consists of
 * "name = value" entries and sections,
 * "[section]"
 * followed by "name = value" entries
 *
 * @category  Configuration
 * @package   Config_Lite
 * @author    Patrick C. Engel <pce@php.net>
 * @copyright 2010-2015 <pce@php.net>
 * @license   http://www.gnu.org/copyleft/lesser.html  LGPL License 2.1
 * @link      https://github.com/pce/config_lite
 */
class Config_Lite implements ArrayAccess, IteratorAggregate, Countable, Serializable
{
    /**
     * sections, holds the config sections
     *
     * @var array
     */
    protected $sections;
    /**
     * filename
     *
     * @var string
     */
    protected $filename;
    /**
     * _booleans - alias of bool in a representable Configuration String Format
     *
     * @var array
     */
    private $_booleans = array('1' => true, 'on' => true,
                               'true' => true, 'yes' => true,
                               '0' => false, 'off' => false,
                               'false' => false, 'no' => false);

    /**
     * line-break chars, default *x: "\n", windows: "\r\n"
     *
     * @var string
     */
    protected $linebreak = "\n";

    /**
     * parseSections - if true, sections will be processed
     *
     * @var bool
     */
    protected $processSections = true;

    /**
     * quote Strings - if true,
     * writes ini files with doublequoted strings
     *
     * @var bool
     */
    protected $quoteStrings = true;

    /**
     * flags for file-put-contents
     *
     * @var int
     */
    protected $flags = 0;
    
    /**
     * string delimiter
     *
     * @var string
     */
    protected $delim = '"';

    /**
     * the read method parses the optional given filename
     * or already setted filename.
     *
     * this method uses the native PHP function
     * parse_ini_file behind the scenes.
     *
     * @param string $filename Filename
     * @param int    $mode     INI_SCANNER_NORMAL | INI_SCANNER_RAW
     *
     * @return Config_Lite
     * @throws Config_Lite_Exception_Runtime when file not found
     * @throws Config_Lite_Exception_Runtime when file is not readable
     * @throws Config_Lite_Exception_Runtime when parse ini file failed
     */
    public function read($filename = null, $mode = INI_SCANNER_NORMAL)
    {
        if (null === $filename) {
            $filename = $this->filename;
        } else {
            $this->filename = $filename;
        }
        if (!file_exists($filename)) {
            throw new Config_Lite_Exception_Runtime('file not found: ' . $filename);
        }
        if (!is_readable($filename)) {
            throw new Config_Lite_Exception_Runtime(
                'file not readable: ' . $filename
            );
        }
        $this->sections = parse_ini_file($filename, $this->processSections, $mode);
        if (false === $this->sections) {
            throw new Config_Lite_Exception_Runtime(
                'failure, can not parse the file: ' . $filename
            );
        }
        return $this;
    }

    /**
     * save the object to the filename
     * (active record style)
     *
     * @return bool
     */
    public function save()
    {
        return $this->write($this->filename, $this->sections, $this->flags);
    }

    /**
     * sync the file to the object
     *
     * like `save',
     * but after written the data, reads the data back into the object.
     * This method is not for the average use-case, ie. for testing.
     *
     * @return Config_Lite
     * @throws Config_Lite_Exception_Runtime when file is not set,
     *         write or readable
     */
    public function sync()
    {
        if (!isset($this->filename)) {
            throw new Config_Lite_Exception_Runtime('no filename set.');
        }
        if (!is_array($this->sections)) {
            $this->sections = array();
        }
        if ($this->write($this->filename, $this->sections)) {
            $this->read($this->filename);
        }
        return $this;
    }

    /**
     * detect Type "bool" by String Value to keep those "untouched"
     *
     * @param string $value value
     *
     * @return bool
     */
    protected function isBool($value)
    {
        return in_array($value, $this->_booleans);
    }

    /**
     * normalize a Value by determining the Type
     *
     * @param string $value value
     *
     * @return string
     */
    protected function normalizeValue($value)
    {
        if (is_bool($value)) {
            $value = $this->toBool($value);
            return $value;
        } elseif (is_numeric($value)) {
            return $value;
        }
        if ($this->quoteStrings) {
            $value = $this->delim . $value . $this->delim;
        }
        return $value;
    }

    /**
     * set string delimiter to single tick (')
     *
     * @return Config_Lite
     */
    public function setSingleTickDelimiter()
    {
        $this->delim = "'";
        return $this;
    }

    /**
     * set string delimiter to double tick (")
     *
     * @return Config_Lite
     */
    public function setDoubleTickDelimiter()
    {
        $this->delim = '"';
        return $this;
    }

    /**
     * Set Flags ( FILE_USE_INCLUDE_PATH | FILE_APPEND | LOCK_EX )
     * for file-put-contents
     *
     * @param int $flags any or binary combined
     *
     * @return Config_Lite
     */
    public function setFlags($flags=LOCK_EX)
    {
        $this->flags = $flags;
        return $this;
    }

    /**
     * generic write ini config file, to save use `save'.
     *
     * writes the global options and sections with normalized Values,
     * that means "bool" values to human readable representation,
     * doublequotes strings and numeric values without any quotes.
     * prepends a php exit if suffix is php,
     * it is valid to write an empty Config file,
     * this method is used by save and is public for explicit usage,
     * eg. if you do not want to hold the whole configuration in the object.
     *
     * @param string $filename      filename
     * @param array  $sectionsarray array with sections
     * @param int    $flags         for file-put-contents
     *
     * @return bool
     * @throws Config_Lite_Exception_Runtime when file is not writeable
     * @throws Config_Lite_Exception_Runtime when write failed
     */
    public function write($filename, $sectionsarray, $flags=null)
    {
        $content = $this->buildOutputString($sectionsarray);
        if (false === file_put_contents($filename, $content, $flags)) {
            throw new Config_Lite_Exception_Runtime(
                sprintf(
                    'failed to write file `%s\' for writing.', $filename
                )
            );
        }
        return true;
    }

    /**
     * Generated the output of the ini file, suitable for echo'ing or
     * writing back to the ini file.
     *
     * @param array $sectionsarray array of ini data
     *
     * @return  string
     */
    protected function buildOutputString($sectionsarray)
    {
        $content = '';
        $sections = '';
        $globals  = '';
        if (!empty($sectionsarray)) {
            // 2 loops to write `globals' on top, alternative: buffer
            foreach ($sectionsarray as $section => $item) {
                if (!is_array($item)) {
                    $value    = $this->normalizeValue($item);
                    $globals .= $section . ' = ' . $value . $this->linebreak;
                }
            }
            $content .= $globals;
            foreach ($sectionsarray as $section => $item) {
                if (is_array($item)) {
                    $sections .= $this->linebreak
                                . "[" . $section . "]" . $this->linebreak;
                    foreach ($item as $key => $value) {
                        if (is_array($value)) {
                            foreach ($value as $arrkey => $arrvalue) {
                                $arrvalue  = $this->normalizeValue($arrvalue);
                                $arrkey    = $key . '[' . $arrkey . ']';
                                $sections .= $arrkey . ' = ' . $arrvalue
                                            . $this->linebreak;
                            }
                        } else {
                            $value     = $this->normalizeValue($value);
                            $sections .= $key . ' = ' . $value . $this->linebreak;
                        }
                    }
                }
            }
            $content .= $sections;
        }
        return $content;
    }

    /**
     * converts string to a  representable Config Bool Format
     *
     * @param string $value value
     *
     * @return string
     * @throws Config_Lite_Exception_UnexpectedValue when format is unknown
     */
    public function toBool($value)
    {
        if ($value === true) {
            return 'yes';
        }
        return 'no';
    }

    /**
     * returns a stripslashed string
     *
     * @param string $sec     Section
     * @param string $key     Key
     * @param mixed  $default default return value
     *
     * @return string
     * @throws Config_Lite_Exception_Runtime when config is empty
     *         and no default value is given
     * @throws Config_Lite_Exception_UnexpectedValue key not found
     *         and no default value is given
     */
    public function getString($sec, $key, $default = null)
    {
        if ((null === $this->sections) && (null === $default)) {
            throw new Config_Lite_Exception_Runtime(
                'configuration seems to be empty, no sections.'
            );
        }
        if ((null === $sec) && array_key_exists($key, $this->sections)) {
            return stripslashes($this->sections[$key]);
        }
        if (array_key_exists($key, $this->sections[$sec])) {
            return stripslashes($this->sections[$sec][$key]);
        }
        if (null !== $default) {
            return $default;
        }
        throw new Config_Lite_Exception_UnexpectedValue(
            'key not found, no default value given.'
        );
    }

    /**
     * get an option by section, a global option or all sections and options
     *
     * to get an option by section, call get with a section and the option.
     * To get a global option call `get' with null as section.
     * Just call `get' without any parameters to get all sections and options.
     * The third parameter is an optional default value to return,
     * if the option is not set, this is practical when dealing with
     * editable files, to keep an application stable with default settings.
     *
     * @param string $sec     Section|null - null to get global option
     * @param string $key     Key
     * @param mixed  $default return default value if is $key is not set
     *
     * @return mixed
     * @throws Config_Lite_Exception when config is empty
     *         and no default value is given
     * @throws Config_Lite_Exception_UnexpectedValue key not found
     *         and no default value is given
     */
    public function get($sec = null, $key = null, $default = null)
    {
        // handle get without parameters, because we can
        if ((null === $sec) && (null === $key) && (null === $default)) {
            return $this; // arrayaccess or $this->sections;
        }
        if ((null !== $sec) && array_key_exists($sec, $this->sections)
            && isset($this->sections[$sec][$key])
        ) {
            return $this->sections[$sec][$key];
        }
        // global value
        if ((null === $sec) && array_key_exists($key, $this->sections)) {
            return $this->sections[$key];
        }
        // section
        if ((null === $key) && array_key_exists($sec, $this->sections)) {
            return $this->sections[$sec];
        }
        // all sections
        if (null === $sec && array_key_exists($sec, $this->sections)) {
            return $this->sections;
        }
        if (null !== $default) {
            return $default;
        }
        throw new Config_Lite_Exception_UnexpectedValue(
            'key not found, no default value given.'
        );
    }

    /**
     * returns a boolean for strict equality comparison
     *
     * returns "on", "yes", 1, "true" as TRUE
     * and no given value or "off", "no", 0, "false" as FALSE
     *
     * @param string $sec     Section
     * @param string $key     Key
     * @param bool   $default return default value if is $key is not set
     *
     * @return bool
     * @throws Config_Lite_Exception_Runtime when the configuration is empty
     *         and no default value is given
     * @throws Config_Lite_Exception_InvalidArgument when is not a boolean
     *         and no default array is given
     * @throws Config_Lite_Exception_UnexpectedValue when key not found
     *         and no default array is given
     */
    public function getBool($sec, $key, $default = null)
    {
        if ((null === $this->sections) && (null === $default)) {
            throw new Config_Lite_Exception_Runtime(
                'configuration seems to be empty (no sections),'
                . 'and no default value given.'
            );
        }
        if ((null === $sec)) {
            if (array_key_exists($key, $this->sections)) {
                if (empty($this->sections[$key])) {
                    return false;
                }
                $value = strtolower($this->sections[$key]);
                if (!in_array($value, $this->_booleans) && (null === $default)) {
                    throw new Config_Lite_Exception_InvalidArgument(
                        sprintf(
                            'Not a boolean: %s, and no default value given.',
                            $value
                        )
                    );
                } else {
                    return $this->_booleans[$value];
                }
            }
        }
        if (array_key_exists($key, $this->sections[$sec])) {
            if (empty($this->sections[$sec][$key])) {
                return false;
            }
            $value = strtolower($this->sections[$sec][$key]);
            if (!in_array($value, $this->_booleans) && (null === $default)) {
                throw new Config_Lite_Exception_InvalidArgument(
                    sprintf(
                        'Not a boolean: %s, and no default value given.',
                        $value
                    )
                );
            } else {
                return $this->_booleans[$value];
            }
        }
        if (null !== $default) {
            return $default;
        }
        throw new Config_Lite_Exception_UnexpectedValue(
            'option not found, no default value given.'
        );
    }

    /**
     * returns an array of options of the given section
     *
     * @param string $sec     Section
     * @param array  $default return default array if $sec is not set
     *
     * @return array
     * @throws Config_Lite_Exception_Runtime when config is empty
     *         and no default array is given
     * @throws Config_Lite_Exception_UnexpectedValue when key not found
     *         and no default array is given
     */
    public function getSection($sec, $default = null)
    {
        if ((null === $this->sections) && (null === $default)) {
            throw new Config_Lite_Exception_Runtime(
                'configuration seems to be empty, no sections.'
            );
        }
        if (isset($this->sections[$sec])) {
            return $this->sections[$sec];
        }
        if ((null !== $default) && is_array($default)) {
            return $default;
        }
        throw new Config_Lite_Exception_UnexpectedValue(
            'section not found, no default array given.'
        );
    }

    /**
     * returns true if the given section exists, otherwise false
     *
     * @param string $sec Section
     *
     * @return bool
     */
    public function hasSection($sec)
    {
        if (isset($this->sections[$sec]) && is_array($this->sections[$sec])) {
            return true;
        }
        return false;
    }

    /**
     * tests if a section or an option of a section exists
     *
     * @param string $sec Section
     * @param string $key Key
     *
     * @return bool
     */
    public function has($sec, $key=null)
    {
        if (!$this->hasSection($sec)) {
            return false;
        }
        if ((null !== $key) && isset($this->sections[$sec][$key])) {
            return true;
        }
        return false;
    }

    /**
     * remove a section or an option of a section
     *
     * @param string $sec Section
     * @param string $key Key
     *
     * @return void
     * @throws Config_Lite_Exception_UnexpectedValue when given Section not exists
     */
    public function remove($sec, $key=null)
    {
        if (null === $key) {
            $this->removeSection($sec);
            return;
        }
        // global value
        if (null === $sec && $key !== null) {
            if (!isset($this->sections[$key])) {
                throw new Config_Lite_Exception_UnexpectedValue(
                    'No such global Value.'
                );
            }
            unset($this->sections[$key]);
            return;
        }
        if (!isset($this->sections[$sec])) {
            throw new Config_Lite_Exception_UnexpectedValue('No such Section.');
        }
        unset($this->sections[$sec][$key]);
    }

    /**
     * remove section by name
     *
     * @param string $sec Section
     *
     * @return void
     * @throws Config_Lite_Exception_UnexpectedValue when given Section not exists
     */
    public function removeSection($sec)
    {
        if (!isset($this->sections[$sec])) {
            throw new Config_Lite_Exception_UnexpectedValue('No such Section.');
        }
        unset($this->sections[$sec]);
    }

    /**
     * removes all sections and global options
     *
     * @return Config_Lite
     */
    public function clear()
    {
        $this->sections = array();
        return $this;
    }

    /**
     * like set, but adds slashes to the value
     *
     * creates new section if necessary and overrides existing keys.
     *
     * @param string $sec   Section
     * @param string $key   Key
     * @param mixed  $value Value
     *
     * @return Config_Lite
     * @throws Config_Lite_Exception_InvalidArgument when given key is an array
     */
    public function setString($sec, $key, $value = null)
    {
        if (null !== $value) {
            $value = addslashes($value);
        }
        $this->set($sec, $key, $value);
        return $this;
    }

    /**
     *  to add key/value pairs
     *
     * creates new section if necessary and overrides existing keys.
     * To set a global, "sectionless" value, call set with null as section.
     *
     * @param string $sec   Section
     * @param string $key   Key
     * @param mixed  $value Value
     *
     * @throws Config_Lite_Exception when given key is an array
     * @return Config_Lite
     */
    public function set($sec, $key, $value = null)
    {
        if (!is_array($this->sections)) {
            $this->sections = array();
        }
        if (is_array($key) || is_array($sec)) {
            throw new Config_Lite_Exception_InvalidArgument(
                'string key expected, but array given.'
            );
        }
        if (null === $sec) {
            $this->sections[$key] = $value;
        } else {
            $this->sections[$sec][$key] = $value;
        }
        return $this;
    }

    /**
     * set a given array with key/value pairs to a section,
     * creates a new section if necessary.
     *
     * @param string $sec   Section
     * @param array  $pairs Keys and Values as Array ('key' => 'value')
     *
     * @throws Config_Lite_Exception_InvalidArgument array $pairs expected
     * @return Config_Lite
     */
    public function setSection($sec, $pairs)
    {
        if (!is_array($this->sections)) {
            $this->sections = array();
        }
        if (!is_array($pairs)) {
            throw new Config_Lite_Exception_InvalidArgument('array expected.');
        }
        $this->sections[$sec] = $pairs;
        return $this;
    }

    /**
     * filename to read or save
     *
     * the full filename with suffix, ie. `[PATH/]<ApplicationName>.ini'.
     * you can also set the filename as parameter to the constructor.
     *
     * @param string $filename Filename
     *
     * @return Config_Lite
     */
    public function setFilename($filename)
    {
        $this->filename = $filename;
        return $this;
    }

    /**
     * returns the current filename
     *
     * @return string
     */
    public function getFilename()
    {
        return $this->filename;
    }

    /**
     * set the line break (newline) chars
     *
     * line-break defaults to Unix Newline "\n",
     * set to support other linebreaks, eg. windows user
     * textfiles "\r\n"
     *
     * @param string $linebreakchars chars
     *
     * @return Config_Lite
     */
    public function setLinebreak($linebreakchars)
    {
        $this->linebreak = $linebreakchars;
        return $this;
    }

    /**
     * Sets whether or not sections should be processed
     *
     * If true, values for each section will be placed into
     * a sub-array for the section. If false, all values will
     * be placed in the global scope.
     *
     * @param bool $processSections - if true, sections will be processed
     *
     * @return Config_Lite
     */
    public function setProcessSections($processSections)
    {
        $this->processSections = $processSections;
        return $this;
    }

    /**
     * Sets whether or not to doubleQuote
     *
     * If true, everything but bool and numeric
     * values get doublequoted.
     *
     * @param bool $quoteStrings - if true, Strings get doubleQuoted
     *
     * @return Config_Lite
     */
    public function setQuoteStrings($quoteStrings)
    {
        $this->quoteStrings = $quoteStrings;
        return $this;
    }

    /**
     * text presentation of the config object
     *
     * since a empty config is valid,
     * it would return a empty string in that case.
     *
     * @throws Config_Lite_Exception_Runtime
     * @return string
     */
    public function __toString()
    {
        return $this->buildOutputString($this->sections);
    }

    /**
     * implemented for interface ArrayAccess
     *
     * @param string $offset section, implemented by ArrayAccess
     * @param mixed  $value  KVP, implemented by ArrayAccess
     *
     * @return void
     */
    public function offsetSet($offset, $value)
    {
        $this->sections[$offset] = $value;
    }

    /**
     * implemented for interface ArrayAccess
     *
     * @param string $offset - section, implemented by ArrayAccess
     *
     * @return bool
     */
    public function offsetExists($offset)
    {
        return isset($this->sections[$offset]);
    }

    /**
     * implemented for interface ArrayAccess
     *
     * @param string $offset - section, implemented by ArrayAccess
     *
     * @return void
     */
    public function offsetUnset($offset)
    {
        unset($this->sections[$offset]);
    }

    /**
     * implemented for interface ArrayAccess
     *
     * @param string $offset - section, implemented by ArrayAccess
     *
     * @return mixed
     */
    public function offsetGet($offset)
    {
        if (array_key_exists($offset, $this->sections)) {
            return $this->sections[$offset];
        }
        return null;
    }
    /**
     * implemented for interface IteratorAggregate
     *
     * @see http://www.php.net/~helly/php/ext/spl/interfaceIterator.html
     * @return Iterator
     */
    public function getIterator()
    {
        return new ArrayIterator($this->sections);
    }

    /**
     * implemented for interface Countable
     *
     * @see http://php.net.countable
     * @return int
     */
    public function count()
    {
        return count($this->sections);
    }

    /**
     * implemented for interface Serializable
     *
     * @see http://php.net/manual/en/class.serializable.php
     * @return int
     */
    public function serialize()
    {
        return serialize($this->sections);
    }

    /**
     * implemented for interface Serializable
     *
     * @param string $serializedData for instance
     *
     * @return void
     */
    public function unserialize($serializedData)
    {
        $sections = unserialize($serializedData);
        $this->sections = $sections;
    }

    /**
     * takes an optional filename, if the file exists, also reads it.
     *
     * the `save' and `read' methods relies on a setted filename,
     * but you can also use `setFilename' to set the filename.
     *
     * @param string $filename - "INI Style" Text Config File
     * @param int    $flags    - flags for file_put_contents, eg. FILE_APPEND
     * @param int    $mode     - set scannermode
     */
    public function __construct($filename = null, $flags = null, $mode = 0)
    {
        $this->sections = array();
        if (null !== $filename) {
            $this->setFilename($filename);
            if (file_exists($filename)) {
                $this->read($filename, $mode);
            }
        }
        if (null !== $flags) {
            $this->setFlags($flags);
        }
    }
}

