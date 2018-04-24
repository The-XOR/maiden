import React, { Component } from 'react';
import { decorators, Treebeard } from 'react-treebeard';
import cx from 'classname';
import treeStyle from './explorer-style';
import treeAnim from './explorer-animation';
import './explorer.css';

import ModalContent from './modal-content';
import ModalRename from './modal-rename';
import IconButton from './icon-button';
import { ICONS } from './svg-icons';


const TreeHeader = (props) => {
    let className = cx('explorer-entry', 'noselect', {'dirty': props.node.modified}, {'active': props.node.active});
    return (
        <span className={className}>
            {props.node.name}
        </span>
    );
};

const TreeToggle = ({style}) => {
    const {height, width} = style;
    const midHeight = height * 0.5;
    const points = `0,0 0,${height} ${width},${midHeight}`;

    return (
        <span className="explorer-tree-toggle" style={style.base}>
            <svg height={height} width={width}>
                <polygon points={points} style={style.arrow} />
            </svg>
        </span>
    );
};

const explorerDecorators = {
    ...decorators,
    Header: TreeHeader,
    Toggle: TreeToggle,
};

// TODO: turn this in a containing component called Section so that individual sections can be collapsed and expanded
const SectionHeader = (props) => {
    let tools = props.tools.map(tool => {
        return (
            <IconButton
                key={tool.name}
                action={() => props.buttonAction(tool.name)}
                icon={tool.icon}
                size="12"
                padding="1"
                color="#e4e4e4"
            />
        );
    });

    return (
        <div className='explorer-header'>
            <span className='section-name'>{props.name}</span>
            <span
                className={cx('section-tools', {'opaque': props.showTools})}
            >
                {tools}
            </span>
        </div>
    );
}

class Section extends Component {
    constructor(props) {
        super(props)
        this.state = {
            showTools: false,
        }
    }

    componentDidMount() {
        this.section.onmouseenter = (e) => {
            this.setState({
                showTools: true,
            })
        }
        this.section.onmouseleave = (e) => {
            this.setState({
                showTools: false,
            })
        }
    }

    render() {
        return (
            <div 
                className='explorer-section'
                ref={(elem) => this.section = elem}
            >
                <SectionHeader
                    name={this.props.name}
                    tools={this.props.tools}
                    buttonAction={this.props.buttonAction}
                    showTools={this.state.showTools}
                />
                <Treebeard
                    style={treeStyle}
                    animations={treeAnim}
                    data={this.props.data}
                    onToggle={this.props.onToggle}
                    decorators={explorerDecorators}
                />
            </div>
        );
    }
}

const scriptTools = [
    {
        name: "add",
        icon: ICONS["plus"],
    },
    {
        name: "remove",
        icon: ICONS["minus"],
    },
    {
        name: "duplicate",
        icon: ICONS["copy"],
    },
    {
        name: "new-folder",
        icon: ICONS["folder-plus"],
    },
    {
        name: "rename",
        icon: ICONS["pencil"],
    },
]

const dataTools = scriptTools;
const audioTools = scriptTools;

class Explorer extends Component {
    onToggle = (node, toggled) => {
        if (node.children) {
            this.props.explorerToggleNode(node, toggled)
            if (toggled) {
                this.props.directoryRead(this.props.api, node.url);
            }
        } else {
            this.props.bufferSelect(node.url);
        }
    }

    onScriptToolClick = (name) => {
        switch (name) {
        case 'add':
            this.props.scriptCreate(this.props.activeBuffer)
            break;

        case 'duplicate':
            this.props.scriptDuplicate(this.props.activeBuffer)
            break;

        case 'remove':
            this.handleRemove()
            break;

        case 'rename':
            this.handleRename()
            break;

        default:
            console.log(name)
            break;
        }
    }

    handleRemove = () => {
        let removeModalCompletion = (choice) => {
            console.log('remove:', choice)
            if (choice === 'ok') {
                this.props.resourceDelete(this.props.api, this.props.activeBuffer)
            }
            this.props.hideModal()
        }

        let scriptName = this.props.activeNode.get("name")
        let content = (
            <ModalContent
                message={`Delete "${scriptName}"?`}
                supporting={"This operation cannot be undone."}
                buttonAction={removeModalCompletion}
            />
        )

        this.props.showModal(content)
    }

    handleRename = () => {
        let complete = (choice, name) => {
            console.log('rename:', choice, name)
            if (name && choice === "ok") {
                this.props.resourceRename(
                    this.props.api,
                    this.props.activeNode.get("url"),
                    name,
                    this.props.activeNode.get("virtual", false)
                )
            }
            this.props.hideModal()
        }

        let initialName = this.props.activeNode.get("name");
        let content = (
            <ModalRename message="Rename" buttonAction={complete} initialName={initialName} />
        )

        this.props.showModal(content)
    }

    render() {
        const {width, height} = this.props;
        return (
            <div className={'explorer' + (this.props.hidden ? ' hidden' : '')} // FIXME: change this to use classname
                 style={{width, height}}
                 ref={(elem) => this.explorer = elem}
            >
                <Section
                    name='scripts'
                    tools={scriptTools}
                    buttonAction={this.onScriptToolClick}
                    data={this.props.data}
                    onToggle={this.onToggle}
                />
                <Section
                    name='data'
                    tools={dataTools}
                    data={this.props.data}
                />
                <Section
                    name='audio'
                    tools={audioTools}
                    data={[]}
                />
            </div>
        );
    }
}

export default Explorer;
