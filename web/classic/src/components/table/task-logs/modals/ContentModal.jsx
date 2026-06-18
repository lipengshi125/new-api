/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState, useEffect } from 'react';
import { Modal, Button, Typography, Spin } from '@douyinfe/semi-ui';
import { IconExternalOpen, IconCopy } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

// 根据 URL 扩展名判断媒体类型
const IMAGE_EXT_REGEX = /\.(png|jpe?g|gif|webp|bmp|svg)(\?|$)/i;
const VIDEO_EXT_REGEX = /\.(mp4|webm|mov|m4v)(\?|$)/i;
const AUDIO_EXT_REGEX = /\.(mp3|wav|ogg|m4a|flac|aac)(\?|$)/i;

const getMediaType = (url) => {
  if (typeof url !== 'string') return 'video';
  if (IMAGE_EXT_REGEX.test(url)) return 'image';
  if (AUDIO_EXT_REGEX.test(url)) return 'audio';
  if (VIDEO_EXT_REGEX.test(url)) return 'video';
  // 未知扩展名（如带鉴权的 API 端点）默认按视频处理，保持旧行为
  return 'video';
};

const ContentModal = ({
  isModalOpen,
  setIsModalOpen,
  modalContent,
  isVideo,
}) => {
  const { t } = useTranslation();
  const [mediaError, setMediaError] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  const mediaType = getMediaType(modalContent);

  useEffect(() => {
    if (isModalOpen && isVideo) {
      setMediaError(false);
      setIsLoading(mediaType !== 'image' ? true : false);
    }
  }, [isModalOpen, isVideo, modalContent, mediaType]);

  const handleMediaError = () => {
    setMediaError(true);
    setIsLoading(false);
  };

  const handleMediaLoaded = () => {
    setIsLoading(false);
  };

  const handleCopyUrl = () => {
    navigator.clipboard.writeText(modalContent);
  };

  const handleOpenInNewTab = () => {
    window.open(modalContent, '_blank');
  };

  const renderMediaContent = () => {
    if (mediaError) {
      const errorTitle =
        mediaType === 'image'
          ? t('图片无法在当前浏览器中加载，这可能是由于：')
          : mediaType === 'audio'
            ? t('音频无法在当前浏览器中播放，这可能是由于：')
            : t('视频无法在当前浏览器中播放，这可能是由于：');
      return (
        <div style={{ textAlign: 'center', padding: '40px' }}>
          <Text
            type='tertiary'
            style={{ display: 'block', marginBottom: '16px' }}
          >
            {errorTitle}
          </Text>
          <Text
            type='tertiary'
            style={{ display: 'block', marginBottom: '8px', fontSize: '12px' }}
          >
            {t('• 视频服务商的跨域限制')}
          </Text>
          <Text
            type='tertiary'
            style={{ display: 'block', marginBottom: '8px', fontSize: '12px' }}
          >
            {t('• 需要特定的请求头或认证')}
          </Text>
          <Text
            type='tertiary'
            style={{ display: 'block', marginBottom: '16px', fontSize: '12px' }}
          >
            {t('• 防盗链保护机制')}
          </Text>

          <div style={{ marginTop: '20px' }}>
            <Button
              icon={<IconExternalOpen />}
              onClick={handleOpenInNewTab}
              style={{ marginRight: '8px' }}
            >
              {t('在新标签页中打开')}
            </Button>
            <Button icon={<IconCopy />} onClick={handleCopyUrl}>
              {t('复制链接')}
            </Button>
          </div>

          <div
            style={{
              marginTop: '16px',
              padding: '8px',
              backgroundColor: '#f8f9fa',
              borderRadius: '4px',
            }}
          >
            <Text
              type='tertiary'
              style={{ fontSize: '10px', wordBreak: 'break-all' }}
            >
              {modalContent}
            </Text>
          </div>
        </div>
      );
    }

    const mediaStyle = {
      width: '100%',
      height: '100%',
      maxWidth: '100%',
      maxHeight: '100%',
      objectFit: 'contain',
    };

    return (
      <div style={{ position: 'relative', height: '100%' }}>
        {isLoading && (
          <div
            style={{
              position: 'absolute',
              top: '50%',
              left: '50%',
              transform: 'translate(-50%, -50%)',
              zIndex: 10,
            }}
          >
            <Spin size='large' />
          </div>
        )}
        {mediaType === 'image' ? (
          <img
            src={modalContent}
            alt={t('预览结果')}
            style={mediaStyle}
            onError={handleMediaError}
            onLoad={handleMediaLoaded}
          />
        ) : mediaType === 'audio' ? (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
            }}
          >
            <audio
              src={modalContent}
              controls
              style={{ width: '100%' }}
              onError={handleMediaError}
              onLoadedData={handleMediaLoaded}
              onLoadStart={() => setIsLoading(true)}
            />
          </div>
        ) : (
          <video
            src={modalContent}
            controls
            style={mediaStyle}
            onError={handleMediaError}
            onLoadedData={handleMediaLoaded}
            onLoadStart={() => setIsLoading(true)}
          />
        )}
      </div>
    );
  };

  return (
    <Modal
      visible={isModalOpen}
      onOk={() => setIsModalOpen(false)}
      onCancel={() => setIsModalOpen(false)}
      closable={null}
      bodyStyle={{
        height: isVideo ? '70vh' : '400px',
        maxHeight: '80vh',
        overflow: 'auto',
        padding: isVideo && mediaError ? '0' : '24px',
      }}
      width={isVideo ? '90vw' : 800}
      style={isVideo ? { maxWidth: 960 } : undefined}
    >
      {isVideo ? (
        renderMediaContent()
      ) : (
        <p style={{ whiteSpace: 'pre-line' }}>{modalContent}</p>
      )}
    </Modal>
  );
};

export default ContentModal;
